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
from itertools import combinations
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
    "STET_CONTEXT_LIMIT": "65536",
    "STET_CRITIC_ENABLED": "false",
    "STET_MAX_COMPLETION_TOKENS": "4096",
    "STET_NUM_CTX": "65536",
    "STET_PROVIDER": "openai",
    "STET_RULES_ANNOTATE": "false",
    "STET_RULES_SELECTOR": "false",
    "STET_STRICTNESS": "default",
    "STET_TEMPERATURE": "0.1",  # never 0: the OpenAI-compat client omits a
    # zero temperature (`omitempty`), so LM Studio's UI value would apply.
    "STET_TIMEOUT": "30m",
}

# Overrides applied on top of PINNED_ENV by `run --critic`; STET_SYSTEMONE_BASE_URL
# comes from --systemone-base-url. Values match .review/config.toml.
CRITIC_ENV: dict[str, str] = {
    "STET_CRITIC_BACKEND": "systemone",
    "STET_CRITIC_ENABLED": "true",
    "STET_SYSTEMONE_MODEL": "openjev-latest",
    "STET_SYSTEMONE_TIMEOUT": "90s",
}
DEFAULT_SYSTEMONE_BASE_URL = "http://127.0.0.1:8080"

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


def pinned_env(model: str, base_url: str, critic_base_url: str | None = None) -> dict[str, str]:
    """Pinned STET_* environment for one stet subprocess (sorted keys).

    When critic_base_url is set, the System One critic is enabled against that
    server so runs measure the critic-on configuration used in .review/config.toml.
    """
    env = dict(PINNED_ENV)
    env["STET_OPENAI_BASE_URL"] = base_url
    env["STET_MODEL"] = model
    if critic_base_url is not None:
        env.update(CRITIC_ENV)
        env["STET_SYSTEMONE_BASE_URL"] = critic_base_url
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
    _validate_real_ref(sha)  # option-looking SHAs could add git arguments
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
    critic_base_url: str | None = None  # None means critic off


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
            _validate_real_ref(item.target)  # option-looking refs could add stet arguments
            clone_and_checkout(opts.repo, item.target, tmp)
            ref = f"{item.target}~1"
        argv = ["stet", "start", ref, "--json", "--stream"]
        if opts.dry_run:
            argv.append("--dry-run")
        _validate_base_url(opts.base_url)  # only http(s) endpoints may be pinned
        env = {**os.environ, **pinned_env(opts.model, opts.base_url, opts.critic_base_url)}
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
        "critic": opts.critic_base_url is not None,
        "env": pinned_env(opts.model, opts.base_url, opts.critic_base_url),
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


def _validate_base_url(base_url: str, flag: str = "--base-url") -> str:
    """Require an http(s) URL so the value cannot smuggle other schemes."""
    if not (base_url.startswith("http://") or base_url.startswith("https://")):
        raise HarnessError(f"{flag} must be an http(s) URL (got {base_url!r})")
    return base_url


def _validate_real_ref(sha: str) -> str:
    """Reject option-looking refs so they cannot add git/stet arguments."""
    if sha.startswith("-"):
        raise HarnessError(f"--real-ref must not start with '-' (got {sha!r})")
    return sha


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
    _validate_base_url(args.base_url)
    critic_base_url = None
    if args.critic:
        critic_base_url = _validate_base_url(args.systemone_base_url, flag="--systemone-base-url")
    real_refs = tuple(_validate_real_ref(sha) for sha in args.real_refs)
    return RunOptions(
        model=model,
        base_url=args.base_url,
        runs=args.runs,
        real_refs=real_refs,
        real_runs=args.real_runs,
        repo=str(args.repo),
        review_timeout_secs=int(args.review_timeout_minutes * 60),
        max_minutes=args.max_minutes,
        retry_failed=args.retry_failed,
        dry_run=args.dry_run,
        cases_dir=Path(args.cases),
        out_dir=Path(args.out),
        stet_version=_stet_version(),
        critic_base_url=critic_base_url,
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
# Matching rule (findings vs. seeded bugs)
# ---------------------------------------------------------------------------


def finding_span(finding: dict) -> tuple[int, int] | None:
    """Inclusive 1-based line span of a finding: range.start..range.end when a
    well-formed ``range`` object is present, else line..line; None when the
    finding carries no line information at all."""
    span_range = finding.get("range")
    if isinstance(span_range, dict):
        start, end = span_range.get("start"), span_range.get("end")
        if (
            isinstance(start, int)
            and not isinstance(start, bool)
            and isinstance(end, int)
            and not isinstance(end, bool)
        ):
            return start, end
    line = finding.get("line")
    if isinstance(line, int) and not isinstance(line, bool):
        return line, line
    return None


def bug_spans_whole_file(bug: Bug, case: Case) -> bool:
    """True when the bug's [start, end] covers every line of the after file."""
    total = case.bug_file_lines.get(bug.file)
    return total is not None and bug.start == 1 and bug.end == total


def finding_matches(finding: dict, bug: Bug, window: int, bug_whole_file: bool = False) -> bool:
    """Match rule: a finding catches a bug when it names the bug's file and its
    line span intersects ``[bug.start - window, bug.end + window]``.

    Edge case (documented per task contract): a finding with no line
    information matches on file alone only when the bug spans the whole file —
    a file-level finding plausibly addresses the only bug only when that bug is
    the entire file content; otherwise it cannot be credited to a specific bug.
    """
    if finding.get("file") != bug.file:
        return False
    span = finding_span(finding)
    if span is None:
        return bug_whole_file
    start, end = span
    return start <= bug.end + window and end >= bug.start - window


def bugs_caught_indices(case: Case, findings: list[dict], window: int) -> set[int]:
    """Indices of the case's bugs caught by at least one finding."""
    caught: set[int] = set()
    for index, bug in enumerate(case.bugs):
        whole = bug_spans_whole_file(bug, case)
        if any(finding_matches(f, bug, window, whole) for f in findings):
            caught.add(index)
    return caught


# ---------------------------------------------------------------------------
# Results tree loading
# ---------------------------------------------------------------------------


@dataclass(frozen=True)
class RecordedRun:
    """One recorded run: its meta plus parsed findings."""

    meta: dict
    findings: list[dict]


@dataclass
class ModelResults:
    """All recorded runs for one model tag."""

    tag: str
    cases: dict[str, dict[int, RecordedRun]]
    real: dict[str, dict[int, RecordedRun]]


_RUN_FILE_RE = re.compile(r"^run(\d+)\.json$")


def load_results(results_dir: str | Path) -> dict[str, ModelResults]:
    """Load every ``<tag>/{cases,real}/<target>/run<N>.json`` under results_dir."""
    results_dir = Path(results_dir)
    if not results_dir.is_dir():
        raise HarnessError(f"results directory not found: {results_dir}")
    models: dict[str, ModelResults] = {}
    for tag_dir in sorted(p for p in results_dir.iterdir() if p.is_dir()):
        model = ModelResults(tag=tag_dir.name, cases={}, real={})
        for group, key in (("cases", "cases"), ("real", "real")):
            group_dir = tag_dir / group
            if not group_dir.is_dir():
                continue
            target_map = getattr(model, key)
            for target_dir in sorted(p for p in group_dir.iterdir() if p.is_dir()):
                runs = _load_recorded_runs(target_dir)
                if runs:
                    target_map[target_dir.name] = runs
        models[tag_dir.name] = model
    return models


def _load_recorded_runs(dir_path: Path) -> dict[int, RecordedRun]:
    runs: dict[int, RecordedRun] = {}
    for entry in sorted(dir_path.iterdir()):
        match = _RUN_FILE_RE.match(entry.name)
        if not match:
            continue
        meta_path = dir_path / f"run{match.group(1)}.meta.json"
        if not meta_path.is_file():
            raise HarnessError(f"missing {meta_path} for findings file {entry}")
        try:
            meta = json.loads(meta_path.read_text(encoding="utf-8"))
            data = json.loads(entry.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError) as exc:
            raise HarnessError(f"unreadable run artifacts in {dir_path}: {exc}") from exc
        if not isinstance(meta, dict):
            raise HarnessError(f"{meta_path} must be a JSON object")
        findings = data.get("findings") if isinstance(data, dict) else None
        if not isinstance(findings, list):
            raise HarnessError(f"{entry} must be a JSON object with a findings list")
        runs[int(match.group(1))] = RecordedRun(meta=meta, findings=findings)
    return runs


# ---------------------------------------------------------------------------
# score subcommand
# ---------------------------------------------------------------------------


def _mean(values: list[float]) -> float:
    return sum(values) / len(values) if values else 0.0


def _round_floats(obj):
    """Recursively round floats to 4 places for deterministic output."""
    if isinstance(obj, float):
        return round(obj, 4)
    if isinstance(obj, dict):
        return {key: _round_floats(value) for key, value in obj.items()}
    if isinstance(obj, list):
        return [_round_floats(value) for value in obj]
    return obj


def _known_ok_runs(model: ModelResults, cases_by_id: dict[str, Case], warn_tag: str) -> list[tuple[Case, int, RecordedRun]]:
    """All ok (case, run_n, run) triples for cases with a known fixture."""
    triples: list[tuple[Case, int, RecordedRun]] = []
    for case_id in sorted(model.cases):
        case = cases_by_id.get(case_id)
        if case is None:
            print(
                f"model_eval: warning: results case {case_id} has no fixture in cases dir; scoring model {warn_tag} without it",
                file=sys.stderr,
            )
            continue
        for run_n in sorted(model.cases[case_id]):
            run = model.cases[case_id][run_n]
            if run.meta.get("status") == STATUS_OK:
                triples.append((case, run_n, run))
    return triples


def _run_recall(
    model: ModelResults, cases: list[Case], cases_by_id: dict[str, Case], window: int, category: str | None
) -> list[float]:
    """Per-run-number recall: each run n is a full pass over the suite; a case
    contributes to run n only when its run n is ok. category=None selects all
    bugs; otherwise only bugs whose category matches."""
    selected: list[tuple[Case, list[int]]] = []
    for case in cases:
        indices = [i for i, bug in enumerate(case.bugs) if category is None or bug.category == category]
        if indices:
            selected.append((case, indices))
    total = sum(len(indices) for _case, indices in selected)
    run_numbers = sorted(
        {
            run_n
            for case_id in model.cases
            if case_id in cases_by_id
            for run_n in model.cases[case_id]
        }
    )
    recalls: list[float] = []
    for run_n in run_numbers:
        caught = 0
        for case, indices in selected:
            run = model.cases.get(case.case_id, {}).get(run_n)
            if run is None or run.meta.get("status") != STATUS_OK:
                continue
            caught += len(bugs_caught_indices(case, run.findings, window) & set(indices))
        recalls.append(caught / total if total else 0.0)
    return recalls


def _finding_buckets(findings: list[dict]) -> set[tuple[str, int]]:
    """Coarse finding identity for repeatability: (file, line // 5). Findings
    without line information share one bucket per file (line -1 // 5 = -1)."""
    buckets: set[tuple[str, int]] = set()
    for f in findings:
        span = finding_span(f)
        line = span[0] if span else -1
        buckets.add((str(f.get("file", "")), line // 5))
    return buckets


def _jaccard(a: set, b: set) -> float:
    union = a | b
    return len(a & b) / len(union) if union else 1.0


def build_model_summary(model: ModelResults, cases_by_id: dict[str, Case], window: int) -> dict:
    """Per-model metrics over the recorded results tree."""
    non_clean = [case for case in sorted(cases_by_id.values(), key=lambda c: c.case_id) if not case.clean]
    total_bugs = sum(len(case.bugs) for case in non_clean)

    recall_mean = _mean(_run_recall(model, non_clean, cases_by_id, window, category=None))

    caught_all: list[str] = []
    caught_some: list[str] = []
    missed_all: list[str] = []
    for case in non_clean:
        ok_findings = [
            run.findings
            for run_n, run in sorted(model.cases.get(case.case_id, {}).items())
            if run.meta.get("status") == STATUS_OK
        ]
        if not ok_findings:
            continue
        per_run = [bugs_caught_indices(case, findings, window) for findings in ok_findings]
        caught_any: set[int] = set()
        for indices in per_run:
            caught_any |= indices
        caught_every = set.intersection(*per_run)
        if len(caught_every) == len(case.bugs):
            caught_all.append(case.case_id)
        elif caught_any:
            caught_some.append(case.case_id)
        else:
            missed_all.append(case.case_id)

    categories = sorted({bug.category for case in non_clean for bug in case.bugs})
    recall_by_category = {}
    for category in categories:
        recalls = _run_recall(model, non_clean, cases_by_id, window, category=category)
        total = sum(1 for case in non_clean for bug in case.bugs if bug.category == category)
        recall_by_category[category] = {"caught_mean": _mean(recalls), "total": total}

    ok_triples = _known_ok_runs(model, cases_by_id, model.tag)
    unexpected_total = 0
    unexpected_by_severity: dict[str, int] = {severity: 0 for severity in SEVERITIES}
    unexpected_on_clean = 0
    for case, _run_n, run in ok_triples:
        for f in run.findings:
            if any(
                finding_matches(f, bug, window, bug_spans_whole_file(bug, case)) for bug in case.bugs
            ):
                continue
            unexpected_total += 1
            severity = str(f.get("severity", "unknown"))
            unexpected_by_severity[severity] = unexpected_by_severity.get(severity, 0) + 1
            if case.clean:
                unexpected_on_clean += 1

    case_jaccards: list[float] = []
    excluded_cases = 0
    for case_id in sorted(set(cases_by_id) | set(model.cases)):
        if case_id not in cases_by_id:
            continue  # results without a fixture are warned about in _known_ok_runs
        bucket_sets = [
            _finding_buckets(run.findings)
            for _run_n, run in sorted(model.cases.get(case_id, {}).items())
            if run.meta.get("status") == STATUS_OK
        ]
        if len(bucket_sets) < 2:
            excluded_cases += 1
            continue
        case_jaccards.append(_mean([_jaccard(a, b) for a, b in combinations(bucket_sets, 2)]))

    reliability = {status: 0 for status in (STATUS_OK, STATUS_FAILED, STATUS_TIMEOUT, STATUS_ZERO_HUNKS)}
    ok_case_walls: list[float] = []
    for case_id in sorted(model.cases):
        for _run_n, run in sorted(model.cases[case_id].items()):
            status = run.meta.get("status")
            if status not in reliability:
                print(
                    f"model_eval: warning: unknown status {status!r} in {model.tag}/cases/{case_id}",
                    file=sys.stderr,
                )
                continue
            reliability[status] += 1
            if status == STATUS_OK:
                wall = run.meta.get("wall_seconds")
                if isinstance(wall, (int, float)):
                    ok_case_walls.append(float(wall))
    ok_real_walls: list[float] = []
    for sha in sorted(model.real):
        for _run_n, run in sorted(model.real[sha].items()):
            status = run.meta.get("status")
            if status not in reliability:
                print(
                    f"model_eval: warning: unknown status {status!r} in {model.tag}/real/{sha}",
                    file=sys.stderr,
                )
                continue
            reliability[status] += 1
            if status == STATUS_OK:
                wall = run.meta.get("wall_seconds")
                if isinstance(wall, (int, float)):
                    ok_real_walls.append(float(wall))

    return {
        "bugs_caught_all_runs": caught_all,
        "bugs_caught_some_runs": caught_some,
        "bugs_missed_all_runs": missed_all,
        "recall_by_category": recall_by_category,
        "recall_mean": recall_mean,
        "repeatability_cases_excluded": excluded_cases,
        "repeatability_jaccard_mean": _mean(case_jaccards),
        "reliability": reliability,
        "seconds_per_case_mean": _mean(ok_case_walls),
        "seconds_per_real_ref_mean": _mean(ok_real_walls),
        "unexpected": {
            "by_severity": unexpected_by_severity,
            "mean_per_run": unexpected_total / len(ok_triples) if ok_triples else 0.0,
            "on_clean_cases": unexpected_on_clean,
            "total": unexpected_total,
        },
    }


def load_blind_key(key_path: str | Path) -> dict:
    """Read and validate the de-anonymization key JSON (anon_id -> entry)."""
    key_path = Path(key_path)
    try:
        key = json.loads(key_path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise HarnessError(f"unreadable blind key {key_path}: {exc}") from exc
    if not isinstance(key, dict):
        raise HarnessError(f"blind key {key_path} must be a JSON object")
    return key


def load_labels(labels_path: str | Path, key_path: str | Path) -> dict[str, str]:
    """Read the labeled blind TSV against the de-anonymization key.

    Returns {anon_id: label} for rows that carry a label. Errors name the
    offending row: unknown anon_ids (not in the key) and unknown labels.
    Rows with an empty label are treated as unlabeled and skipped.
    """
    labels_path = Path(labels_path)
    key = load_blind_key(key_path)
    try:
        text = labels_path.read_text(encoding="utf-8")
    except OSError as exc:
        raise HarnessError(f"unreadable labels file {labels_path}: {exc}") from exc
    labels: dict[str, str] = {}
    for lineno, line in enumerate(text.splitlines(), 1):
        if not line.strip():
            continue
        cols = line.split("\t")
        anon_id = cols[0].strip()
        if anon_id == "anon_id":
            continue  # header
        if not anon_id:
            raise HarnessError(f"labels row {lineno}: missing anon_id")
        if anon_id not in key:
            raise HarnessError(f"labels row {lineno}: unknown anon_id '{anon_id}'")
        label = cols[8].strip() if len(cols) > 8 else ""
        if not label:
            continue  # unlabeled row; excluded from real_diff counts
        if label not in VALID_LABELS:
            raise HarnessError(f"labels row {lineno}: unknown label '{label}' (anon_id {anon_id})")
        labels[anon_id] = label
    return labels


def attach_real_diff(
    summary: dict, labels_by_anon: dict[str, str], blind_key: dict
) -> None:
    """Add real_diff (per-label counts + actionable_rate) to each model entry."""
    counts_by_tag: dict[str, dict[str, int]] = {}
    for anon_id in sorted(labels_by_anon):
        entry = blind_key.get(anon_id)
        model_tag = str(entry.get("model_tag", "")) if isinstance(entry, dict) else ""
        if model_tag not in summary:
            print(
                f"model_eval: warning: labels anon_id {anon_id} maps to model tag '{model_tag}' absent from results",
                file=sys.stderr,
            )
        tag_counts = counts_by_tag.setdefault(
            model_tag, {label: 0 for label in VALID_LABELS}
        )
        tag_counts[labels_by_anon[anon_id]] += 1
    for model_tag, metrics in summary.items():
        tag_counts = counts_by_tag.get(
            model_tag, {label: 0 for label in VALID_LABELS}
        )
        labeled = sum(tag_counts.values())
        actionable_rate = tag_counts[LABEL_ACTIONABLE] / labeled if labeled else 0.0
        metrics["real_diff"] = {
            "actionable_rate": actionable_rate,
            "labeled": labeled,
            "labels": {label: tag_counts[label] for label in VALID_LABELS},
        }


def build_summary(
    cases_by_id: dict[str, Case],
    models: dict[str, ModelResults],
    window: int,
    labels_by_anon: dict[str, str] | None,
    blind_key: dict | None,
) -> dict:
    """Deterministic score summary keyed by model tag."""
    summary = {
        tag: build_model_summary(model, cases_by_id, window) for tag, model in sorted(models.items())
    }
    if labels_by_anon is not None:
        if blind_key is None:  # pragma: no cover - guarded by cmd_score
            raise HarnessError("blind key required with labels")
        attach_real_diff(summary, labels_by_anon, blind_key)
    return _round_floats(summary)


def cmd_score(args: argparse.Namespace) -> int:
    """Compare recorded runs against fixture ground truth; emit JSON summary."""
    try:
        if (args.labels is None) != (args.blind_key is None):
            raise HarnessError("--labels and --blind-key must be used together")
        if args.window < 0:
            raise HarnessError("--window must be >= 0")
        cases = load_cases(args.cases)
        cases_by_id = {case.case_id: case for case in cases}
        models = load_results(args.results)
        labels_by_anon = load_labels(args.labels, args.blind_key) if args.labels else None
        # build_summary/attach_real_diff need the key dict itself, not its path
        # (regression: a Path here crashed with AttributeError on .get).
        blind_key = load_blind_key(args.blind_key) if args.blind_key else None
        summary = build_summary(cases_by_id, models, args.window, labels_by_anon, blind_key)
        text = json.dumps(summary, sort_keys=True, indent=2) + "\n"
        if args.out is not None:
            args.out.parent.mkdir(parents=True, exist_ok=True)
            args.out.write_text(text, encoding="utf-8")
        if args.check is not None:
            try:
                expected = json.loads(Path(args.check).read_text(encoding="utf-8"))
            except (OSError, json.JSONDecodeError) as exc:
                raise HarnessError(f"unreadable check file {args.check}: {exc}") from exc
            if expected == summary:
                print("model_eval: check ok", flush=True)
                return EXIT_OK
            differing = sorted(
                key for key in set(expected) | set(summary) if expected.get(key) != summary.get(key)
            )
            print(
                f"model_eval: check mismatch; differing top-level keys: {', '.join(differing)}",
                flush=True,
            )
            return EXIT_CHECK_MISMATCH
        print(text, end="")
        return EXIT_OK
    except HarnessError as exc:
        print(f"model_eval: error: {exc}", file=sys.stderr)
        return EXIT_ERROR


# ---------------------------------------------------------------------------
# export-blind subcommand
# ---------------------------------------------------------------------------


def _tsv_cell(value) -> str:
    """TSV-safe cell text: tabs and newlines collapse to spaces."""
    return str(value).replace("\t", " ").replace("\r", " ").replace("\n", " ")


def collect_real_findings(models: dict[str, ModelResults]) -> list[tuple[str, str, int, int, dict]]:
    """Every (model_tag, ref, run, index, finding) from real runs, in a
    deterministic pre-shuffle order."""
    items: list[tuple[str, str, int, int, dict]] = []
    for tag in sorted(models):
        for sha in sorted(models[tag].real):
            for run_n in sorted(models[tag].real[sha]):
                for index, f in enumerate(models[tag].real[sha][run_n].findings):
                    items.append((tag, sha, run_n, index, f))
    return items


def cmd_export_blind(args: argparse.Namespace) -> int:
    """Export real-ref findings to an anonymized TSV plus a de-anonymization key."""
    try:
        models = load_results(args.results)
        items = collect_real_findings(models)
        random.Random(args.seed).shuffle(items)
        key: dict[str, dict] = {}
        rows: list[list[str]] = [list(BLIND_TSV_COLUMNS)]
        for position, (tag, sha, run_n, index, f) in enumerate(items, 1):
            anon_id = f"F{position:03d}"
            key[anon_id] = {"model_tag": tag, "ref": sha, "run": run_n, "index": index}
            span = finding_span(f)
            line = f.get("line")
            if not isinstance(line, int) or isinstance(line, bool):
                line = span[0] if span else ""
            rows.append(
                [
                    anon_id,
                    _tsv_cell(sha),
                    _tsv_cell(f.get("file", "")),
                    str(line),
                    _tsv_cell(f.get("severity", "")),
                    _tsv_cell(f.get("category", "")),
                    _tsv_cell(f.get("message", "")),
                    _tsv_cell(f.get("suggestion", "")),
                    "",
                    "",
                ]
            )
        tsv_text = "\n".join("\t".join(row) for row in rows) + "\n"
        args.out.parent.mkdir(parents=True, exist_ok=True)
        args.out.write_text(tsv_text, encoding="utf-8")
        args.key.parent.mkdir(parents=True, exist_ok=True)
        args.key.write_text(json.dumps(key, sort_keys=True, indent=2) + "\n", encoding="utf-8")
        print(f"model_eval: exported {len(items)} findings to {args.out}", flush=True)
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
    run_parser.add_argument("--critic", action="store_true", help="Enable the System One critic (default off)")
    run_parser.add_argument(
        "--systemone-base-url",
        default=DEFAULT_SYSTEMONE_BASE_URL,
        help=f"System One critic server URL, used with --critic (default {DEFAULT_SYSTEMONE_BASE_URL})",
    )

    score_parser = sub.add_parser("score", help="Score recorded runs against fixture ground truth")
    score_parser.add_argument("--cases", type=Path, required=True, help="Fixture cases directory")
    score_parser.add_argument("--results", type=Path, required=True, help="Results directory written by run")
    score_parser.add_argument("--labels", type=Path, default=None, help="Labeled blind TSV (with --blind-key)")
    score_parser.add_argument("--blind-key", type=Path, default=None, help="De-anonymization key JSON (with --labels)")
    score_parser.add_argument("--out", type=Path, default=None, help="Also write the summary JSON to this file")
    score_parser.add_argument("--check", type=Path, default=None, help="Compare summary to this JSON file; exit 2 on diff")
    score_parser.add_argument("--window", type=int, default=DEFAULT_MATCH_WINDOW, help="Line match window (default 3)")

    export_parser = sub.add_parser("export-blind", help="Export real findings to an anonymized TSV for blind triage")
    export_parser.add_argument("--results", type=Path, required=True, help="Results directory written by run")
    export_parser.add_argument("--out", type=Path, required=True, help="Output blind TSV path")
    export_parser.add_argument("--key", type=Path, required=True, help="Output de-anonymization key JSON path")
    export_parser.add_argument("--seed", type=int, default=DEFAULT_BLIND_SEED, help="Shuffle seed (default 20261010)")
    return parser


def main(argv: list[str] | None = None) -> int:
    args = build_parser().parse_args(argv)
    if args.command == "run":
        return cmd_run(args)
    if args.command == "score":
        return cmd_score(args)
    if args.command == "export-blind":
        return cmd_export_blind(args)
    print(f"model_eval: error: unknown command: {args.command}", file=sys.stderr)
    return EXIT_ERROR


if __name__ == "__main__":
    sys.exit(main())
