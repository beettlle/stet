#!/usr/bin/env python3
"""Model comparison harness for stet.

Runs stet with a chosen LLM over seeded-bug fixture cases (see
``docs/efficacy-tests.md`` H2) and real commits, then scores the results so two
models can be compared on the same inputs (efficacy tests A1 and A4).

Subcommands:

- ``run``           materialize temp repos and record one stet review per
                    (target, run) into ``<out>/<model-tag>/...``.
- ``score``         compare recorded runs against fixture ground truth and
                    emit a deterministic JSON summary.
- ``export-blind``  export real-ref findings to an anonymized TSV for the
                    blind triage study (H1) plus a de-anonymization key.

Python 3.14, standard library only. stet and git are invoked as
subprocesses; tests inject fakes for the module-level ``run_simple`` and
``run_stream`` seams and pass a fake clock to ``cmd_run`` instead of sleeping.
"""

from __future__ import annotations

import argparse
import json
import os
import random
import re
import shutil
import signal
import subprocess
import sys
import tempfile
import time
from dataclasses import dataclass
from pathlib import Path
from shutil import which

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

EXIT_OK = 0
EXIT_ERROR = 1  # usage / setup errors (bad args, missing dirs, invalid fixtures)
EXIT_CHECK_MISMATCH = 2  # score --check found differing top-level keys
EXIT_BUDGET_REACHED = 3  # run stopped because --max-minutes elapsed

DEFAULT_BASE_URL = "http://127.0.0.1:1234/v1"
DEFAULT_RUNS = 3
DEFAULT_REAL_RUNS = 1
DEFAULT_REVIEW_TIMEOUT_MINUTES = 30.0
DEFAULT_MATCH_WINDOW = 3
DEFAULT_BLIND_SEED = 20261010

MODEL_TAG_DRY_RUN = "dry-run"

STATUS_OK = "ok"
STATUS_FAILED = "failed"
STATUS_TIMEOUT = "timeout"
STATUS_ZERO_HUNKS = "zero_hunks"

LABEL_ACTIONABLE = "actionable"
LABEL_FALSE_POSITIVE = "false_positive"
LABEL_WRONG_SUGGESTION = "wrong_suggestion"
LABEL_OUT_OF_SCOPE = "out_of_scope"
VALID_LABELS = (LABEL_ACTIONABLE, LABEL_FALSE_POSITIVE, LABEL_WRONG_SUGGESTION, LABEL_OUT_OF_SCOPE)

SEVERITIES = ("error", "warning", "info", "nitpick")

BLIND_TSV_COLUMNS = ("anon_id", "ref", "file", "line", "severity", "category", "message", "suggestion", "label", "notes")

# Git identity flags so commits work without any global git config.
GIT_IDENTITY = ("-c", "user.name=model-eval", "-c", "user.email=model-eval@localhost", "-c", "commit.gpgsign=false")

# Pinned environment applied to every stet subprocess (in addition to the
# caller's environment). Env beats `.review/config.toml`, so a cloned repo's
# tracked config cannot change these. STET_OPENAI_BASE_URL and STET_MODEL are
# added per invocation by pinned_env(); keep PINNED_ENV itself sorted by name.
PINNED_ENV: dict[str, str] = {
    "STET_CONTEXT_LIMIT": "32768",
    "STET_CRITIC_ENABLED": "false",
    "STET_MAX_COMPLETION_TOKENS": "8192",
    "STET_NUM_CTX": "32768",
    "STET_PROVIDER": "openai",
    "STET_RULES_ANNOTATE": "false",
    "STET_RULES_SELECTOR": "false",
    "STET_STRICTNESS": "default",
    "STET_TEMPERATURE": "0.1",  # never 0: the OpenAI-compat client omits a
    # zero temperature (`omitempty`), so LM Studio's UI value would apply.
    "STET_TIMEOUT": "30m",
}

_HUNKS_RE = re.compile(r"(\d+) hunks to review")

# ---------------------------------------------------------------------------
# Errors and injectable subprocess seams
# ---------------------------------------------------------------------------


class HarnessError(Exception):
    """Usage or setup error; reported on stderr and mapped to exit 1."""


@dataclass(frozen=True)
class StreamResult:
    """Outcome of one streamed stet subprocess."""

    returncode: int
    stdout: str
    stderr: str
    timed_out: bool = False


def run_simple(argv: list[str], cwd: str | Path | None = None) -> tuple[int, str, str]:
    """Run a subprocess to completion; return (returncode, stdout, stderr).

    Injectable seam: tests replace this to avoid real git/stet calls.
    """
    proc = subprocess.run(argv, cwd=cwd, capture_output=True, text=True, check=False)
    return proc.returncode, proc.stdout, proc.stderr


def run_stream(argv: list[str], cwd: str | Path, env: dict[str, str], timeout_secs: int) -> StreamResult:
    """Run a subprocess whose stdout is a stream, enforcing a wall timeout.

    On timeout the whole process group is killed. Injectable seam: tests
    replace this to simulate stet output without invoking stet.
    """
    proc = subprocess.Popen(
        argv,
        cwd=cwd,
        env=env,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        start_new_session=True,
    )
    timed_out = False
    try:
        stdout, stderr = proc.communicate(timeout=timeout_secs)
    except subprocess.TimeoutExpired:
        timed_out = True
        try:
            os.killpg(os.getpgid(proc.pid), signal.SIGKILL)
        except (ProcessLookupError, PermissionError, AttributeError):
            proc.kill()
        stdout, stderr = proc.communicate()
    return StreamResult(
        returncode=proc.returncode if proc.returncode is not None else -1,
        stdout=stdout or "",
        stderr=stderr or "",
        timed_out=timed_out,
    )


# ---------------------------------------------------------------------------
# Fixture loading and validation
# ---------------------------------------------------------------------------


@dataclass(frozen=True)
class Bug:
    """One seeded bug from a case's expected.json."""

    file: str
    start: int
    end: int
    category: str
    description: str


@dataclass(frozen=True)
class Case:
    """One fixture case directory."""

    case_id: str
    path: Path
    language: str
    clean: bool
    bugs: tuple[Bug, ...]
    # Line counts of the after/ versions of each bug's file (whole-file rule).
    bug_file_lines: dict[str, int]


def load_cases(cases_dir: str | Path) -> list[Case]:
    """Load and validate every case directory under cases_dir, sorted by id."""
    cases_dir = Path(cases_dir)
    if not cases_dir.is_dir():
        raise HarnessError(f"cases directory not found: {cases_dir}")
    cases = [_load_case(path) for path in sorted(p for p in cases_dir.iterdir() if p.is_dir())]
    return cases


def _load_case(case_path: Path) -> Case:
    case_id = case_path.name
    expected_path = case_path / "expected.json"
    if not expected_path.is_file():
        raise HarnessError(f"case {case_id}: missing expected.json")
    try:
        data = json.loads(expected_path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise HarnessError(f"case {case_id}: unreadable expected.json: {exc}") from exc
    if not isinstance(data, dict):
        raise HarnessError(f"case {case_id}: expected.json must be a JSON object")
    for field in ("id", "language", "clean", "bugs"):
        if field not in data:
            raise HarnessError(f"case {case_id}: expected.json missing field '{field}'")
    if data["id"] != case_id:
        raise HarnessError(f"case {case_id}: expected.json id {data['id']!r} does not match directory name")
    if not isinstance(data["language"], str) or not data["language"]:
        raise HarnessError(f"case {case_id}: language must be a non-empty string")
    if not isinstance(data["clean"], bool):
        raise HarnessError(f"case {case_id}: clean must be a boolean")
    if not isinstance(data["bugs"], list):
        raise HarnessError(f"case {case_id}: bugs must be a list")

    bugs: list[Bug] = []
    for index, raw in enumerate(data["bugs"]):
        bugs.append(_load_bug(case_id, index, raw))
    if data["clean"] and bugs:
        raise HarnessError(f"case {case_id}: clean is true but {len(bugs)} bug(s) are listed")

    before_dir = case_path / "before"
    after_dir = case_path / "after"
    if not before_dir.is_dir():
        raise HarnessError(f"case {case_id}: missing before/ directory")
    if not after_dir.is_dir():
        raise HarnessError(f"case {case_id}: missing after/ directory")

    bug_file_lines: dict[str, int] = {}
    for bug in bugs:
        if bug.file in bug_file_lines:
            continue
        after_file = after_dir / f"{bug.file}.txt"
        if not after_file.is_file():
            raise HarnessError(f"case {case_id}: bug file {bug.file} missing from after/")
        try:
            bug_file_lines[bug.file] = len(after_file.read_text(encoding="utf-8").splitlines())
        except OSError as exc:
            raise HarnessError(f"case {case_id}: unreadable after/{bug.file}.txt: {exc}") from exc

    return Case(
        case_id=case_id,
        path=case_path,
        language=data["language"],
        clean=data["clean"],
        bugs=tuple(bugs),
        bug_file_lines=bug_file_lines,
    )


def _load_bug(case_id: str, index: int, raw: object) -> Bug:
    label = f"bug #{index}"
    if not isinstance(raw, dict):
        raise HarnessError(f"case {case_id}: {label} must be a JSON object")
    for field in ("file", "lines", "category", "description"):
        if field not in raw:
            raise HarnessError(f"case {case_id}: {label} missing field '{field}'")
    file_name = raw["file"]
    if not isinstance(file_name, str) or not file_name:
        raise HarnessError(f"case {case_id}: {label} file must be a non-empty string")
    lines = raw["lines"]
    if (
        not isinstance(lines, list)
        or len(lines) != 2
        or not all(isinstance(value, int) and not isinstance(value, bool) for value in lines)
    ):
        raise HarnessError(f"case {case_id}: {label} lines must be a 1-based [start, end] pair")
    start, end = lines
    if start > end:
        raise HarnessError(f"case {case_id}: {label} lines start {start} is greater than end {end}")
    if start < 1:
        raise HarnessError(f"case {case_id}: {label} lines must be 1-based (got start {start})")
    if not isinstance(raw["category"], str) or not isinstance(raw["description"], str):
        raise HarnessError(f"case {case_id}: {label} category and description must be strings")
    return Bug(file=file_name, start=start, end=end, category=raw["category"], description=raw["description"])


# ---------------------------------------------------------------------------
# Environment
# ---------------------------------------------------------------------------


def pinned_env(model: str, base_url: str) -> dict[str, str]:
    """Pinned STET_* environment for one stet subprocess (sorted keys)."""
    env = dict(PINNED_ENV)
    env["STET_OPENAI_BASE_URL"] = base_url
    env["STET_MODEL"] = model
    return env


def model_tag(model: str) -> str:
    """Directory name for a model: '/' replaced by '__'."""
    return model.replace("/", "__")


# ---------------------------------------------------------------------------
# Temp repo materialization
# ---------------------------------------------------------------------------


def _copy_fixture_tree(src_dir: Path, dest_root: Path) -> None:
    """Copy ``*.txt`` files from src_dir into dest_root, stripping ``.txt``."""
    if not src_dir.is_dir():
        return
    for src_file in sorted(src_dir.rglob("*.txt")):
        if src_file.suffix != ".txt":
            continue  # rglob("*.txt") is case-insensitive on macOS; only exact .txt is fixture schema
        rel = src_file.relative_to(src_dir)
        dest = dest_root / rel.with_suffix("")
        dest.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(src_file, dest)


def _git(argv: list[str], cwd: str | Path, what: str) -> None:
    returncode, _stdout, stderr = run_simple(argv, cwd=cwd)
    if returncode != 0:
        raise HarnessError(f"{what} failed (exit {returncode}): {stderr.strip()}")


def materialize_case_repo(case: Case, tmp: str) -> None:
    """Turn a fixture case into a two-commit git repo inside tmp."""
    _git(["git", *GIT_IDENTITY, "init", "--quiet"], cwd=tmp, what=f"git init for case {case.case_id}")
    _copy_fixture_tree(case.path / "before", Path(tmp))
    _git(["git", *GIT_IDENTITY, "add", "-A"], cwd=tmp, what=f"git add (before) for case {case.case_id}")
    _git(
        ["git", *GIT_IDENTITY, "commit", "--quiet", "--allow-empty", "-m", "model-eval: before"],
        cwd=tmp,
        what=f"git commit (before) for case {case.case_id}",
    )
    _copy_fixture_tree(case.path / "after", Path(tmp))
    _git(["git", *GIT_IDENTITY, "add", "-A"], cwd=tmp, what=f"git add (after) for case {case.case_id}")
    _git(
        ["git", *GIT_IDENTITY, "commit", "--quiet", "--allow-empty", "-m", "model-eval: after"],
        cwd=tmp,
        what=f"git commit (after) for case {case.case_id}",
    )


def clone_and_checkout(repo: str | Path, sha: str, tmp: str) -> None:
    """Clone repo into tmp and detach to sha (never touches the source repo)."""
    _git(["git", "clone", "--quiet", "--no-hardlinks", str(repo), tmp], cwd=None, what=f"git clone of {repo}")
    _git(["git", "checkout", "--quiet", "--detach", sha], cwd=tmp, what=f"git checkout {sha}")


# ---------------------------------------------------------------------------
# Stream parsing and review execution
# ---------------------------------------------------------------------------


@dataclass
class ReviewOutcome:
    """Result of one stet review invocation."""

    status: str
    findings: list[dict]
    hunks_total: int
    exit_code: int
    wall_seconds: float
    stderr_text: str


def parse_stream(stdout: str) -> tuple[int, list[dict], bool]:
    """Parse stet --json --stream NDJSON output.

    Returns (hunks_total, findings, malformed). ``progress`` events matching
    "<N> hunks to review" set hunks_total, ``finding`` events append their
    ``data`` object, ``done`` ends the stream, and unknown event types are
    ignored. Any line that is not a JSON object marks the stream malformed.
    """
    hunks_total = 0
    findings: list[dict] = []
    for line in stdout.splitlines():
        line = line.strip()
        if not line:
            continue
        try:
            event = json.loads(line)
        except json.JSONDecodeError:
            return hunks_total, findings, True
        if not isinstance(event, dict):
            return hunks_total, findings, True
        event_type = event.get("type")
        if event_type == "progress":
            msg = event.get("msg")
            if isinstance(msg, str):
                match = _HUNKS_RE.search(msg)
                if match:
                    hunks_total = int(match.group(1))
        elif event_type == "finding":
            data = event.get("data")
            if isinstance(data, dict):
                findings.append(data)
        elif event_type == "done":
            break
        # Unknown event types are ignored.
    return hunks_total, findings, False


@dataclass(frozen=True)
class RunOptions:
    """Validated options for the run subcommand."""

    model: str
    base_url: str
    runs: int
    real_refs: tuple[str, ...]
    real_runs: int
    repo: str
    review_timeout_secs: int
    max_minutes: float | None
    retry_failed: bool
    dry_run: bool
    cases_dir: Path
    out_dir: Path
    stet_version: str


@dataclass(frozen=True)
class WorkItem:
    """One (kind, target, run) review to execute."""

    kind: str  # "case" | "real"
    target: str
    run_n: int
    case: Case | None


@dataclass(frozen=True)
class RunPaths:
    """Output file paths for one work item."""

    findings: Path
    meta: Path
    stderr_log: Path


def _run_paths(out_root: Path, item: WorkItem) -> RunPaths:
    base = out_root / ("cases" if item.kind == "case" else "real") / item.target
    stem = f"run{item.run_n}"
    return RunPaths(
        findings=base / f"{stem}.json",
        meta=base / f"{stem}.meta.json",
        stderr_log=base / f"{stem}.stderr.log",
    )


def _stet_version() -> str:
    returncode, stdout, _stderr = run_simple(["stet", "--version"])
    if returncode != 0:
        return "unknown"
    text = stdout.strip()
    return text.splitlines()[0] if text else "unknown"


def perform_review(item: WorkItem, opts: RunOptions, clock=time.monotonic) -> ReviewOutcome:
    """Materialize the temp repo, run one stet review, and clean up.

    The stet subprocess gets the pinned environment; on timeout the process
    group is killed and ``stet finish`` is still attempted. The temp repo is
    always removed. exit_code is -1 when the review timed out.
    """
    tmp = tempfile.mkdtemp(prefix="stet-model-eval-")
    stderr_text = ""
    try:
        if item.kind == "case":
            assert item.case is not None
            materialize_case_repo(item.case, tmp)
            ref = "HEAD~1"
        else:
            clone_and_checkout(opts.repo, item.target, tmp)
            ref = f"{item.target}~1"
        argv = ["stet", "start", ref, "--json", "--stream"]
        if opts.dry_run:
            argv.append("--dry-run")
        env = {**os.environ, **pinned_env(opts.model, opts.base_url)}
        started = clock()
        result = run_stream(argv, cwd=tmp, env=env, timeout_secs=opts.review_timeout_secs)
        wall_seconds = clock() - started
        hunks_total, findings, malformed = parse_stream(result.stdout)
        if result.timed_out:
            status = STATUS_TIMEOUT
        elif malformed:
            status = STATUS_FAILED
        elif result.returncode != 0:
            status = STATUS_FAILED
        elif hunks_total == 0:
            status = STATUS_ZERO_HUNKS
        else:
            status = STATUS_OK
        exit_code = -1 if result.timed_out else result.returncode
        stderr_text = result.stderr
    finally:
        finish_code, _finish_out, finish_err = run_simple(["stet", "finish"], cwd=tmp)
        if finish_code != 0:
            note = finish_err.strip() or f"exit {finish_code}"
            stderr_text = (stderr_text or "") + f"\n# stet finish failed: {note}\n"
        shutil.rmtree(tmp, ignore_errors=True)
    return ReviewOutcome(
        status=status,
        findings=findings,
        hunks_total=hunks_total,
        exit_code=exit_code,
        wall_seconds=wall_seconds,
        stderr_text=stderr_text,
    )


def _build_meta(item: WorkItem, opts: RunOptions, outcome: ReviewOutcome) -> dict:
    return {
        "env": pinned_env(opts.model, opts.base_url),
        "exit_code": outcome.exit_code,
        "findings_count": len(outcome.findings),
        "hunks_total": outcome.hunks_total,
        "kind": item.kind,
        "model": opts.model,
        "run": item.run_n,
        "status": outcome.status,
        "stet_version": opts.stet_version,
        "target": item.target,
        "wall_seconds": round(outcome.wall_seconds, 4),
    }


def _write_run_artifacts(paths: RunPaths, item: WorkItem, opts: RunOptions, outcome: ReviewOutcome) -> None:
    paths.findings.parent.mkdir(parents=True, exist_ok=True)
    paths.findings.write_text(json.dumps({"findings": outcome.findings}, sort_keys=True, indent=2) + "\n", encoding="utf-8")
    paths.meta.write_text(json.dumps(_build_meta(item, opts, outcome), sort_keys=True, indent=2) + "\n", encoding="utf-8")
    paths.stderr_log.write_text(outcome.stderr_text or "", encoding="utf-8")


def _load_meta_status(meta_path: Path) -> str:
    """Existing meta status, or '' when unreadable (corrupt meta re-runs)."""
    try:
        meta = json.loads(meta_path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError):
        print(f"model_eval: warning: unreadable meta, re-running: {meta_path}", file=sys.stderr)
        return ""
    status = meta.get("status")
    return status if isinstance(status, str) else ""


# ---------------------------------------------------------------------------
# run subcommand
# ---------------------------------------------------------------------------


def _validate_tooling() -> None:
    if which("git") is None:
        raise HarnessError("git not found on PATH")
    if which("stet") is None:
        raise HarnessError("stet not found on PATH")


def _run_options_from_args(args: argparse.Namespace) -> RunOptions:
    if args.dry_run:
        model = args.model or MODEL_TAG_DRY_RUN
    elif not args.model:
        raise HarnessError("--model is required unless --dry-run")
    else:
        model = args.model
    if args.runs < 1:
        raise HarnessError("--runs must be >= 1")
    if args.real_runs < 0:
        raise HarnessError("--real-runs must be >= 0")
    if args.review_timeout_minutes <= 0:
        raise HarnessError("--review-timeout-minutes must be > 0")
    _validate_tooling()
    return RunOptions(
        model=model,
        base_url=args.base_url,
        runs=args.runs,
        real_refs=tuple(args.real_refs),
        real_runs=args.real_runs,
        repo=str(args.repo),
        review_timeout_secs=int(args.review_timeout_minutes * 60),
        max_minutes=args.max_minutes,
        retry_failed=args.retry_failed,
        dry_run=args.dry_run,
        cases_dir=Path(args.cases),
        out_dir=Path(args.out),
        stet_version=_stet_version(),
    )


def _work_items(opts: RunOptions, cases: list[Case]) -> list[WorkItem]:
    items: list[WorkItem] = []
    for run_n in range(1, opts.runs + 1):
        items.extend(WorkItem(kind="case", target=case.case_id, run_n=run_n, case=case) for case in cases)
    for run_n in range(1, opts.real_runs + 1):
        items.extend(WorkItem(kind="real", target=sha, run_n=run_n, case=None) for sha in opts.real_refs)
    return items


def cmd_run(args: argparse.Namespace, clock=time.monotonic) -> int:
    """Run stet over every (target, run) work item, resuming where possible."""
    try:
        opts = _run_options_from_args(args)
        cases = load_cases(opts.cases_dir)
        if not cases and not opts.real_refs:
            print(
                f"model_eval: warning: no cases found in {opts.cases_dir}; running real refs only",
                file=sys.stderr,
            )
        tag = model_tag(opts.model)
        out_root = opts.out_dir / tag
        items = _work_items(opts, cases)
        total = len(items)
        remaining = total
        started = clock()
        for position, item in enumerate(items, 1):
            paths = _run_paths(out_root, item)
            if paths.meta.exists():
                status = _load_meta_status(paths.meta)
                if not opts.retry_failed or status == STATUS_OK:
                    remaining -= 1
                    continue
            if opts.max_minutes is not None and (clock() - started) > opts.max_minutes * 60:
                print(f"model_eval: budget reached; remaining={remaining}", flush=True)
                return EXIT_BUDGET_REACHED
            outcome = perform_review(item, opts, clock)
            _write_run_artifacts(paths, item, opts, outcome)
            remaining -= 1
            print(
                f"[{position}/{total}] {tag} {item.kind} {item.target} run{item.run_n}"
                f" -> {outcome.status} {len(outcome.findings)} findings {outcome.wall_seconds:.1f}s",
                flush=True,
            )
        print("model_eval: done; remaining=0", flush=True)
        return EXIT_OK
    except HarnessError as exc:
        print(f"model_eval: error: {exc}", file=sys.stderr)
        return EXIT_ERROR


# ---------------------------------------------------------------------------
# CLI
# ---------------------------------------------------------------------------


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="model_eval.py", description="Model comparison harness for stet")
    sub = parser.add_subparsers(dest="command", required=True)

    run_parser = sub.add_parser("run", help="Run stet over fixture cases and real refs")
    run_parser.add_argument("--model", help="Model name (required unless --dry-run)")
    run_parser.add_argument("--cases", type=Path, required=True, help="Fixture cases directory")
    run_parser.add_argument("--out", type=Path, required=True, help="Results output directory")
    run_parser.add_argument("--runs", type=int, default=DEFAULT_RUNS, help="Runs per fixture case (default 3)")
    run_parser.add_argument(
        "--real-ref",
        dest="real_refs",
        action="append",
        default=[],
        metavar="SHA",
        help="Real commit SHA to review (repeatable)",
    )
    run_parser.add_argument("--real-runs", type=int, default=DEFAULT_REAL_RUNS, help="Runs per real ref (default 1)")
    run_parser.add_argument("--repo", type=Path, default=Path("."), help="Source repo to clone real refs from")
    run_parser.add_argument("--base-url", default=DEFAULT_BASE_URL, help="OpenAI-compatible base URL")
    run_parser.add_argument("--max-minutes", type=float, default=None, help="Wall budget in minutes")
    run_parser.add_argument(
        "--review-timeout-minutes",
        type=float,
        default=DEFAULT_REVIEW_TIMEOUT_MINUTES,
        help="Per-review subprocess timeout in minutes (default 30)",
    )
    run_parser.add_argument("--retry-failed", action="store_true", help="Re-run runs whose status is not ok")
    run_parser.add_argument("--dry-run", action="store_true", help="Pass --dry-run to stet start (no LLM)")
    return parser


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    if args.command == "run":
        return cmd_run(args)
    print(f"model_eval: error: unknown command: {args.command}", file=sys.stderr)
    return EXIT_ERROR


if __name__ == "__main__":
    sys.exit(main())
