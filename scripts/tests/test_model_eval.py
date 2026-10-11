"""Tests for scripts.model_eval (SP-037 model comparison harness)."""

from __future__ import annotations

import argparse
import json
import shutil
import subprocess
import sys
from pathlib import Path

import pytest

from scripts import model_eval


# ---------------------------------------------------------------------------
# Helpers and fakes
# ---------------------------------------------------------------------------


class FakeClock:
    """Monotonic clock advancing a fixed step per call."""

    def __init__(self, step: float = 10.0) -> None:
        self.step = step
        self.value = 0.0

    def __call__(self) -> float:
        self.value += self.step
        return self.value


class FakeSimple:
    """Replaces model_eval.run_simple: records calls, returns success."""

    def __init__(self, fail: set[str] | None = None) -> None:
        self.calls: list[tuple[list[str], str | None]] = []
        self.fail = fail or set()

    def __call__(self, argv: list[str], cwd=None):
        self.calls.append((list(argv), str(cwd) if cwd else None))
        joined = " ".join(argv)
        if joined in self.fail:
            return 1, "", f"fake failure: {joined}"
        if "version" in joined:  # stet --version
            return 0, "stet version fake\n", ""
        return 0, "", ""


class FakeRunner:
    """Replaces model_eval.run_stream: records calls, returns canned results."""

    def __init__(self, results: list[model_eval.StreamResult] | None = None) -> None:
        self.calls: list[dict] = []
        self.results = list(results or [])
        self.default = model_eval.StreamResult(returncode=0, stdout="", stderr="")

    def __call__(self, argv, cwd, env, timeout_secs):
        self.calls.append({"argv": list(argv), "cwd": str(cwd), "env": dict(env), "timeout_secs": timeout_secs})
        if self.results:
            return self.results.pop(0)
        return self.default


def make_case(
    root: Path,
    case_id: str = "go-off-by-one",
    clean: bool = False,
    bugs: list[dict] | None = None,
    expected_extra: dict | None = None,
    expected_missing: list[str] | None = None,
) -> Path:
    """Write a fixture case directory and return its path."""
    case_dir = root / case_id
    (case_dir / "before" / "internal" / "page").mkdir(parents=True)
    (case_dir / "after" / "internal" / "page").mkdir(parents=True)
    (case_dir / "before" / "internal" / "page" / "page.go.txt").write_text("package page\n\nfunc f() {\n}\n", encoding="utf-8")
    (case_dir / "after" / "internal" / "page" / "page.go.txt").write_text(
        "package page\n\nfunc f() {\n\tx := make([]int, 0, len(s)+1)\n}\n", encoding="utf-8"
    )
    if bugs is None and not clean:
        bugs = [
            {
                "file": "internal/page/page.go",
                "lines": [4, 4],
                "category": "correctness",
                "description": "slice end uses len+1",
            }
        ]
    expected = {"id": case_id, "language": "go", "clean": clean, "bugs": bugs or []}
    expected.update(expected_extra or {})
    for field in expected_missing or []:
        expected.pop(field, None)
    (case_dir / "expected.json").write_text(json.dumps(expected), encoding="utf-8")
    return case_dir


def make_run_args(tmp_path: Path, **overrides) -> argparse.Namespace:
    defaults = dict(
        model="test/model-a",
        cases=tmp_path / "cases",
        out=tmp_path / "out",
        runs=1,
        real_refs=[],
        real_runs=1,
        repo=tmp_path / "repo",
        base_url=model_eval.DEFAULT_BASE_URL,
        max_minutes=None,
        review_timeout_minutes=30.0,
        retry_failed=False,
        dry_run=False,
    )
    defaults.update(overrides)
    return argparse.Namespace(**defaults)


def finding(file: str = "internal/page/page.go", line: int = 4, severity: str = "error") -> dict:
    return {
        "id": "f1",
        "file": file,
        "line": line,
        "severity": severity,
        "category": "correctness",
        "confidence": 0.9,
        "message": "off by one",
    }


def stream_result(
    findings: list[dict] | None = None,
    hunks_msg: str | None = "2 hunks to review",
    returncode: int = 0,
    timed_out: bool = False,
    raw: str | None = None,
) -> model_eval.StreamResult:
    if raw is None:
        lines = []
        if hunks_msg:
            lines.append(json.dumps({"type": "progress", "msg": hunks_msg}))
        for f in findings or []:
            lines.append(json.dumps({"type": "finding", "data": f}))
        lines.append(json.dumps({"type": "done"}))
        raw = "\n".join(lines) + "\n"
    return model_eval.StreamResult(returncode=returncode, stdout=raw, stderr="stderr-noise", timed_out=timed_out)


@pytest.fixture
def fake_tools(monkeypatch: pytest.MonkeyPatch) -> FakeSimple:
    """Patch tool discovery and simple subprocesses; run_stream stays real (unused)."""
    simple = FakeSimple()
    monkeypatch.setattr(model_eval, "run_simple", simple)
    monkeypatch.setattr(model_eval, "which", lambda name: f"/fake-bin/{name}")
    return simple


# ---------------------------------------------------------------------------
# Fixture loading and validation
# ---------------------------------------------------------------------------


class TestLoadCases:
    def test_missing_cases_dir_raises(self, tmp_path: Path) -> None:
        with pytest.raises(model_eval.HarnessError, match="cases directory not found"):
            model_eval.load_cases(tmp_path / "nope")

    def test_valid_case_loads_sorted(self, tmp_path: Path) -> None:
        make_case(tmp_path / "cases", case_id="b-case")
        make_case(tmp_path / "cases", case_id="a-case", clean=True, bugs=None)
        cases = model_eval.load_cases(tmp_path / "cases")
        assert [case.case_id for case in cases] == ["a-case", "b-case"]
        buggy = cases[1]
        assert buggy.bugs[0].file == "internal/page/page.go"
        assert (buggy.bugs[0].start, buggy.bugs[0].end) == (4, 4)
        assert buggy.bug_file_lines["internal/page/page.go"] == 5
        assert cases[0].clean and cases[0].bugs == ()

    @pytest.mark.parametrize(
        ("kwargs", "match"),
        [
            ({"expected_missing": ["language"]}, "missing field 'language'"),
            ({"expected_extra": {"id": "other-id"}}, "does not match directory name"),
        ],
    )
    def test_invalid_expected_json_names_case(self, tmp_path: Path, kwargs: dict, match: str) -> None:
        make_case(tmp_path / "cases", case_id="go-off-by-one", **kwargs)
        with pytest.raises(model_eval.HarnessError, match="case go-off-by-one"):
            model_eval.load_cases(tmp_path / "cases")

    def test_lines_start_gt_end_names_case(self, tmp_path: Path) -> None:
        make_case(
            tmp_path / "cases",
            case_id="go-off-by-one",
            bugs=[{"file": "internal/page/page.go", "lines": [16, 14], "category": "correctness", "description": "d"}],
        )
        with pytest.raises(model_eval.HarnessError, match=r"case go-off-by-one: bug #0"):
            model_eval.load_cases(tmp_path / "cases")

    def test_clean_with_bugs_names_case(self, tmp_path: Path) -> None:
        make_case(
            tmp_path / "cases",
            case_id="go-off-by-one",
            clean=True,
            bugs=[{"file": "internal/page/page.go", "lines": [1, 2], "category": "correctness", "description": "d"}],
        )
        with pytest.raises(model_eval.HarnessError, match="case go-off-by-one"):
            model_eval.load_cases(tmp_path / "cases")

    def test_bug_file_missing_from_after(self, tmp_path: Path) -> None:
        make_case(
            tmp_path / "cases",
            case_id="go-off-by-one",
            bugs=[{"file": "nope/missing.go", "lines": [1, 1], "category": "correctness", "description": "d"}],
        )
        with pytest.raises(model_eval.HarnessError, match="missing from after/"):
            model_eval.load_cases(tmp_path / "cases")

    def test_missing_expected_json(self, tmp_path: Path) -> None:
        case_dir = make_case(tmp_path / "cases")
        (case_dir / "expected.json").unlink()
        with pytest.raises(model_eval.HarnessError, match="missing expected.json"):
            model_eval.load_cases(tmp_path / "cases")


# ---------------------------------------------------------------------------
# Pinned environment
# ---------------------------------------------------------------------------


class TestPinnedEnv:
    def test_pinned_env_contains_contract_values(self) -> None:
        env = model_eval.pinned_env("qwen/qwen3-coder-next", "http://127.0.0.1:1234/v1")
        assert env["STET_PROVIDER"] == "openai"
        assert env["STET_OPENAI_BASE_URL"] == "http://127.0.0.1:1234/v1"
        assert env["STET_MODEL"] == "qwen/qwen3-coder-next"
        assert env["STET_TEMPERATURE"] == "0.1"
        assert env["STET_MAX_COMPLETION_TOKENS"] == "4096"
        assert env["STET_NUM_CTX"] == "65536"
        assert env["STET_CONTEXT_LIMIT"] == "65536"
        assert env["STET_STRICTNESS"] == "default"
        assert env["STET_CRITIC_ENABLED"] == "false"
        assert env["STET_RULES_SELECTOR"] == "false"
        assert env["STET_RULES_ANNOTATE"] == "false"
        assert env["STET_TIMEOUT"] == "30m"

    def test_pinned_env_overrides_caller_environment(self, monkeypatch: pytest.MonkeyPatch) -> None:
        monkeypatch.setenv("STET_TEMPERATURE", "0.9")
        env = model_eval.pinned_env("m", "http://x/v1")
        assert env["STET_TEMPERATURE"] == "0.1"

    def test_model_tag(self) -> None:
        assert model_eval.model_tag("qwen/qwen3-coder-next") == "qwen__qwen3-coder-next"
        assert model_eval.model_tag("dry-run") == "dry-run"


# ---------------------------------------------------------------------------
# Stream parsing
# ---------------------------------------------------------------------------


class TestParseStream:
    def test_progress_finding_done(self) -> None:
        stdout = "\n".join(
            [
                json.dumps({"type": "progress", "msg": "3 hunks to review"}),
                json.dumps({"type": "progress", "msg": "Reviewing hunk 1/3: a.go"}),
                json.dumps({"type": "finding", "data": finding()}),
                json.dumps({"type": "done"}),
            ]
        )
        hunks, findings, malformed = model_eval.parse_stream(stdout)
        assert hunks == 3
        assert findings == [finding()]
        assert malformed is False

    def test_unknown_event_type_ignored(self) -> None:
        stdout = "\n".join(
            [
                json.dumps({"type": "progress", "msg": "1 hunks to review"}),
                json.dumps({"type": "hunk_count", "count": 7}),
                json.dumps({"type": "finding", "data": finding()}),
                json.dumps({"type": "done"}),
            ]
        )
        hunks, findings, malformed = model_eval.parse_stream(stdout)
        assert hunks == 1
        assert len(findings) == 1
        assert malformed is False

    def test_malformed_line_marks_stream_failed(self) -> None:
        hunks, findings, malformed = model_eval.parse_stream(
            json.dumps({"type": "progress", "msg": "1 hunks to review"}) + "\nnot-json\n"
        )
        assert hunks == 1
        assert findings == []
        assert malformed is True

    def test_non_object_event_is_malformed(self) -> None:
        _hunks, _findings, malformed = model_eval.parse_stream("[1, 2]\n")
        assert malformed is True

    def test_zero_hunks_when_no_progress(self) -> None:
        hunks, findings, malformed = model_eval.parse_stream(json.dumps({"type": "done"}) + "\n")
        assert hunks == 0
        assert findings == []
        assert malformed is False


# ---------------------------------------------------------------------------
# Temp repo materialization (real git)
# ---------------------------------------------------------------------------


@pytest.mark.skipif(shutil.which("git") is None, reason="git not on PATH")
class TestMaterializeCaseRepo:
    def test_materialize_two_commits_with_stripped_txt(self, tmp_path: Path) -> None:
        case = model_eval.load_cases(make_case(tmp_path / "cases").parent)[0]
        tmp_repo = tmp_path / "repo"
        tmp_repo.mkdir()
        model_eval.materialize_case_repo(case, str(tmp_repo))

        log = subprocess.run(
            ["git", "log", "--format=%s", "--name-status"], cwd=tmp_repo, capture_output=True, text=True, check=True
        ).stdout
        # Newest commit first: after-commit adds the modified file, before-commit adds the base file.
        assert "model-eval: after" in log
        assert "model-eval: before" in log
        assert (tmp_repo / "internal" / "page" / "page.go").is_file()
        assert not (tmp_repo / "internal" / "page" / "page.go.txt").exists()
        diff = subprocess.run(
            ["git", "diff", "HEAD~1", "HEAD", "--name-only"], cwd=tmp_repo, capture_output=True, text=True, check=True
        ).stdout
        assert diff.strip() == "internal/page/page.go"

    def test_added_file_only_in_after(self, tmp_path: Path) -> None:
        case_dir = make_case(tmp_path / "cases")
        added = case_dir / "after" / "internal" / "page" / "extra.go.txt"
        added.write_text("package page\n", encoding="utf-8")
        case = model_eval.load_cases(case_dir.parent)[0]
        tmp_repo = tmp_path / "repo"
        tmp_repo.mkdir()
        model_eval.materialize_case_repo(case, str(tmp_repo))
        assert (tmp_repo / "internal" / "page" / "extra.go").is_file()
        files = subprocess.run(
            ["git", "diff", "HEAD~1", "HEAD", "--name-only"], cwd=tmp_repo, capture_output=True, text=True, check=True
        ).stdout.split()
        assert sorted(files) == ["internal/page/extra.go", "internal/page/page.go"]

    def test_git_failure_raises(self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
        case = model_eval.load_cases(make_case(tmp_path / "cases").parent)[0]
        monkeypatch.setattr(model_eval, "run_simple", lambda argv, cwd=None: (128, "", "boom"))
        with pytest.raises(model_eval.HarnessError, match="git init"):
            model_eval.materialize_case_repo(case, str(tmp_path / "repo"))


# ---------------------------------------------------------------------------
# run subcommand (faked subprocesses)
# ---------------------------------------------------------------------------


class TestCmdRunHappyPath:
    def test_two_case_runs_and_one_real_ref(
        self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
    ) -> None:
        make_case(tmp_path / "cases")
        monkeypatch.setattr(model_eval, "run_simple", FakeSimple())
        runner = FakeRunner([stream_result([finding()]), stream_result([]), stream_result([finding(), finding()])])
        monkeypatch.setattr(model_eval, "run_stream", runner)
        code = model_eval.cmd_run(
            make_run_args(tmp_path, runs=2, real_refs=["abc123"]), clock=FakeClock(step=1.0)
        )

        assert code == model_eval.EXIT_OK
        out_dir = tmp_path / "out" / "test__model-a"
        # Two case runs + one real run
        for path in [
            out_dir / "cases" / "go-off-by-one" / "run1.json",
            out_dir / "cases" / "go-off-by-one" / "run1.meta.json",
            out_dir / "cases" / "go-off-by-one" / "run1.stderr.log",
            out_dir / "cases" / "go-off-by-one" / "run2.json",
            out_dir / "real" / "abc123" / "run1.json",
            out_dir / "real" / "abc123" / "run1.meta.json",
        ]:
            assert path.is_file(), path

        meta = json.loads((out_dir / "cases" / "go-off-by-one" / "run1.meta.json").read_text())
        assert meta["model"] == "test/model-a"
        assert meta["kind"] == "case"
        assert meta["target"] == "go-off-by-one"
        assert meta["run"] == 1
        assert meta["status"] == "ok"
        assert meta["exit_code"] == 0
        assert meta["hunks_total"] == 2
        assert meta["findings_count"] == 1
        assert meta["stet_version"] == "stet version fake"
        assert meta["env"]["STET_MODEL"] == "test/model-a"
        assert meta["env"]["STET_TEMPERATURE"] == "0.1"
        run_json = json.loads((out_dir / "cases" / "go-off-by-one" / "run1.json").read_text())
        assert run_json == {"findings": [finding()]}
        real_meta = json.loads((out_dir / "real" / "abc123" / "run1.meta.json").read_text())
        assert real_meta["kind"] == "real"
        assert real_meta["target"] == "abc123"
        assert (out_dir / "cases" / "go-off-by-one" / "run1.stderr.log").read_text() == "stderr-noise"

        # Review order: all case runs first, then real refs.
        refs = [call["argv"][2] for call in runner.calls]
        assert refs == ["HEAD~1", "HEAD~1", "abc123~1"]

        lines = capsys.readouterr().out.strip().splitlines()
        assert lines[0] == "[1/3] test__model-a case go-off-by-one run1 -> ok 1 findings 1.0s"
        assert lines[1] == "[2/3] test__model-a case go-off-by-one run2 -> ok 0 findings 1.0s"
        assert lines[2] == "[3/3] test__model-a real abc123 run1 -> ok 2 findings 1.0s"
        assert lines[-1] == "model_eval: done; remaining=0"

    def test_pinned_env_and_dry_run_flag_reach_subprocess(
        self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch
    ) -> None:
        make_case(tmp_path / "cases")
        monkeypatch.setattr(model_eval, "run_simple", FakeSimple())
        runner = FakeRunner()
        monkeypatch.setattr(model_eval, "run_stream", runner)
        monkeypatch.setenv("STET_TEMPERATURE", "0.9")  # caller env must be overridden
        code = model_eval.cmd_run(make_run_args(tmp_path, dry_run=True, model=None), clock=FakeClock())
        assert code == model_eval.EXIT_OK
        call = runner.calls[0]
        assert "--dry-run" in call["argv"]
        assert call["argv"][:2] == ["stet", "start"]
        assert call["env"]["STET_PROVIDER"] == "openai"
        assert call["env"]["STET_TEMPERATURE"] == "0.1"
        assert call["env"]["STET_MODEL"] == "dry-run"
        assert call["env"]["STET_OPENAI_BASE_URL"] == model_eval.DEFAULT_BASE_URL
        assert "PATH" in call["env"]
        assert (tmp_path / "out" / "dry-run" / "cases" / "go-off-by-one" / "run1.meta.json").is_file()

    def test_review_timeout_reaches_subprocess(self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
        make_case(tmp_path / "cases")
        monkeypatch.setattr(model_eval, "run_simple", FakeSimple())
        runner = FakeRunner()
        monkeypatch.setattr(model_eval, "run_stream", runner)
        model_eval.cmd_run(make_run_args(tmp_path, review_timeout_minutes=0.5), clock=FakeClock())
        assert runner.calls[0]["timeout_secs"] == 30


class TestCmdRunStatuses:
    def test_nonzero_exit_is_failed_and_artifacts_written(
        self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
    ) -> None:
        make_case(tmp_path / "cases")
        monkeypatch.setattr(model_eval, "run_simple", FakeSimple())
        monkeypatch.setattr(
            model_eval, "run_stream", FakeRunner([stream_result(returncode=2, raw="")])
        )
        code = model_eval.cmd_run(make_run_args(tmp_path), clock=FakeClock())
        assert code == model_eval.EXIT_OK
        out = tmp_path / "out" / "test__model-a" / "cases" / "go-off-by-one"
        meta = json.loads((out / "run1.meta.json").read_text())
        assert meta["status"] == "failed"
        assert meta["exit_code"] == 2
        assert (out / "run1.json").is_file()
        assert (out / "run1.stderr.log").read_text() == "stderr-noise"
        assert "-> failed 0 findings 10.0s" in capsys.readouterr().out

    def test_timeout_kills_and_reports(self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
        make_case(tmp_path / "cases")
        monkeypatch.setattr(model_eval, "run_simple", FakeSimple())
        monkeypatch.setattr(
            model_eval,
            "run_stream",
            FakeRunner([stream_result(findings=[finding()], timed_out=True, returncode=-9)]),
        )
        model_eval.cmd_run(make_run_args(tmp_path), clock=FakeClock())
        meta = json.loads(
            (tmp_path / "out" / "test__model-a" / "cases" / "go-off-by-one" / "run1.meta.json").read_text()
        )
        assert meta["status"] == "timeout"
        assert meta["exit_code"] == -1
        assert meta["findings_count"] == 1

    def test_zero_hunks(self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
        make_case(tmp_path / "cases")
        monkeypatch.setattr(model_eval, "run_simple", FakeSimple())
        monkeypatch.setattr(
            model_eval, "run_stream", FakeRunner([stream_result(raw=json.dumps({"type": "done"}) + "\n")])
        )
        model_eval.cmd_run(make_run_args(tmp_path), clock=FakeClock())
        meta = json.loads(
            (tmp_path / "out" / "test__model-a" / "cases" / "go-off-by-one" / "run1.meta.json").read_text()
        )
        assert meta["status"] == "zero_hunks"
        assert meta["hunks_total"] == 0

    def test_malformed_stream_is_failed_not_crash(self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
        make_case(tmp_path / "cases")
        monkeypatch.setattr(model_eval, "run_simple", FakeSimple())
        monkeypatch.setattr(model_eval, "run_stream", FakeRunner([stream_result(raw="garbage line\n")]))
        model_eval.cmd_run(make_run_args(tmp_path), clock=FakeClock())
        meta = json.loads(
            (tmp_path / "out" / "test__model-a" / "cases" / "go-off-by-one" / "run1.meta.json").read_text()
        )
        assert meta["status"] == "failed"

    def test_finish_failure_is_recorded_in_stderr_log(self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
        make_case(tmp_path / "cases")
        simple = FakeSimple(fail={"stet finish"})
        monkeypatch.setattr(model_eval, "run_simple", simple)
        monkeypatch.setattr(model_eval, "run_stream", FakeRunner([stream_result([finding()])]))
        model_eval.cmd_run(make_run_args(tmp_path), clock=FakeClock())
        log = (tmp_path / "out" / "test__model-a" / "cases" / "go-off-by-one" / "run1.stderr.log").read_text()
        assert "stet finish failed" in log
        assert "fake failure: stet finish" in log


class TestCmdRunResumeAndBudget:
    def test_resume_skips_existing_meta(self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
        make_case(tmp_path / "cases")
        simple = FakeSimple()
        monkeypatch.setattr(model_eval, "run_simple", simple)
        runner = FakeRunner()
        monkeypatch.setattr(model_eval, "run_stream", runner)
        args = make_run_args(tmp_path)
        assert model_eval.cmd_run(args, clock=FakeClock()) == model_eval.EXIT_OK
        first_calls = len(runner.calls)
        assert first_calls == 1
        # Second invocation: meta exists -> skip entirely.
        assert model_eval.cmd_run(args, clock=FakeClock()) == model_eval.EXIT_OK
        assert len(runner.calls) == first_calls

    def test_retry_failed_reruns_non_ok_only(self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
        make_case(tmp_path / "cases")
        monkeypatch.setattr(model_eval, "run_simple", FakeSimple())
        runner = FakeRunner([stream_result(returncode=1, raw=""), stream_result([finding()])])
        monkeypatch.setattr(model_eval, "run_stream", runner)
        args = make_run_args(tmp_path, retry_failed=True)
        assert model_eval.cmd_run(args, clock=FakeClock()) == model_eval.EXIT_OK
        meta = json.loads(
            (tmp_path / "out" / "test__model-a" / "cases" / "go-off-by-one" / "run1.meta.json").read_text()
        )
        assert meta["status"] == "failed"
        assert len(runner.calls) == 1
        # Second pass with --retry-failed re-runs the failed run and now succeeds.
        runner2 = FakeRunner([stream_result([finding()])])
        monkeypatch.setattr(model_eval, "run_stream", runner2)
        assert model_eval.cmd_run(args, clock=FakeClock()) == model_eval.EXIT_OK
        assert len(runner2.calls) == 1
        meta = json.loads(
            (tmp_path / "out" / "test__model-a" / "cases" / "go-off-by-one" / "run1.meta.json").read_text()
        )
        assert meta["status"] == "ok"

    def test_budget_reached_exits_3_with_remaining(
        self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch, capsys: pytest.CaptureFixture[str]
    ) -> None:
        make_case(tmp_path / "cases")
        monkeypatch.setattr(model_eval, "run_simple", FakeSimple())
        monkeypatch.setattr(model_eval, "run_stream", FakeRunner())
        code = model_eval.cmd_run(make_run_args(tmp_path, runs=3, max_minutes=1.0), clock=FakeClock(step=10.0))
        assert code == model_eval.EXIT_BUDGET_REACHED
        out_text = capsys.readouterr().out
        assert "model_eval: budget reached; remaining=1" in out_text
        assert "model_eval: done" not in out_text
        # Two reviews completed before the budget triggered.
        cases_dir = tmp_path / "out" / "test__model-a" / "cases" / "go-off-by-one"
        assert (cases_dir / "run1.meta.json").is_file()
        assert (cases_dir / "run2.meta.json").is_file()
        assert not (cases_dir / "run3.meta.json").exists()


class TestCmdRunSetupErrors:
    def test_missing_cases_dir_exits_1(self, tmp_path: Path, fake_tools: FakeSimple, capsys: pytest.CaptureFixture[str]) -> None:
        code = model_eval.cmd_run(make_run_args(tmp_path), clock=FakeClock())
        assert code == model_eval.EXIT_ERROR
        assert "error" in capsys.readouterr().err

    def test_model_required_unless_dry_run(self, tmp_path: Path, fake_tools: FakeSimple) -> None:
        assert model_eval.cmd_run(make_run_args(tmp_path, model=None), clock=FakeClock()) == model_eval.EXIT_ERROR

    def test_invalid_expected_json_exits_1_with_case_id(
        self, tmp_path: Path, fake_tools: FakeSimple, capsys: pytest.CaptureFixture[str]
    ) -> None:
        make_case(
            tmp_path / "cases",
            bugs=[{"file": "internal/page/page.go", "lines": [9, 4], "category": "correctness", "description": "d"}],
        )
        code = model_eval.cmd_run(make_run_args(tmp_path), clock=FakeClock())
        assert code == model_eval.EXIT_ERROR
        assert "case go-off-by-one" in capsys.readouterr().err

    @pytest.mark.parametrize("tool", ["git", "stet"])
    def test_missing_tool_exits_1(self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch, tool: str) -> None:
        monkeypatch.setattr(model_eval, "run_simple", FakeSimple())
        monkeypatch.setattr(model_eval, "which", lambda name, _tool=tool: None if name == _tool else f"/bin/{name}")
        code = model_eval.cmd_run(make_run_args(tmp_path), clock=FakeClock())
        assert code == model_eval.EXIT_ERROR

    def test_invalid_runs(self, tmp_path: Path, fake_tools: FakeSimple) -> None:
        assert model_eval.cmd_run(make_run_args(tmp_path, runs=0), clock=FakeClock()) == model_eval.EXIT_ERROR


# ---------------------------------------------------------------------------
# Step 2: score helpers
# ---------------------------------------------------------------------------


def f_at(file: str, line: int | None = None, severity: str = "error", rng: tuple[int, int] | None = None) -> dict:
    result: dict = {"file": file, "severity": severity, "category": "correctness", "message": "m"}
    if rng is not None:
        result["range"] = {"start": rng[0], "end": rng[1]}
    elif line is not None:
        result["line"] = line
    return result


class TestFindingMatches:
    BUG = model_eval.Bug(file="a.go", start=10, end=12, category="correctness", description="d")

    def test_exact_window_edges_match(self) -> None:
        assert model_eval.finding_matches(f_at("a.go", 10), self.BUG, 3)
        assert model_eval.finding_matches(f_at("a.go", 12), self.BUG, 3)
        assert model_eval.finding_matches(f_at("a.go", 7), self.BUG, 3)  # start - window
        assert model_eval.finding_matches(f_at("a.go", 15), self.BUG, 3)  # end + window

    def test_one_line_beyond_window_does_not_match(self) -> None:
        assert not model_eval.finding_matches(f_at("a.go", 6), self.BUG, 3)
        assert not model_eval.finding_matches(f_at("a.go", 16), self.BUG, 3)

    def test_range_overlap_matches_by_intersection(self) -> None:
        assert model_eval.finding_matches(f_at("a.go", rng=(11, 40)), self.BUG, 3)
        assert model_eval.finding_matches(f_at("a.go", rng=(14, 20)), self.BUG, 3)
        assert not model_eval.finding_matches(f_at("a.go", rng=(1, 4)), self.BUG, 3)  # ends before 10-3
        assert not model_eval.finding_matches(f_at("a.go", rng=(1, 2)), self.BUG, 3)
        assert not model_eval.finding_matches(f_at("a.go", rng=(20, 30)), self.BUG, 3)

    def test_wrong_file_never_matches(self) -> None:
        assert not model_eval.finding_matches(f_at("b.go", 10), self.BUG, 3)
        assert not model_eval.finding_matches(f_at("b.go", 11), self.BUG, 3)

    def test_no_line_finding_matches_only_whole_file_bugs(self) -> None:
        no_line = f_at("a.go")
        assert not model_eval.finding_matches(no_line, self.BUG, 3, bug_whole_file=False)
        assert model_eval.finding_matches(no_line, self.BUG, 3, bug_whole_file=True)

    def test_bug_spans_whole_file(self, tmp_path: Path) -> None:
        case_dir = make_case(
            tmp_path / "cases",
            case_id="whole",
            bugs=[{"file": "internal/page/page.go", "lines": [1, 5], "category": "correctness", "description": "d"}],
        )
        case = model_eval.load_cases(case_dir.parent)[0]
        assert case.bug_file_lines["internal/page/page.go"] == 5
        assert model_eval.bug_spans_whole_file(case.bugs[0], case)
        partial_bug = model_eval.Bug(file="internal/page/page.go", start=1, end=4, category="c", description="d")
        assert not model_eval.bug_spans_whole_file(partial_bug, case)
        # End-to-end: a no-line finding on that file catches the whole-file bug.
        assert model_eval.bugs_caught_indices(case, [f_at("internal/page/page.go")], 3) == {0}

    def test_finding_span_prefers_range_over_line(self) -> None:
        assert model_eval.finding_span({"file": "a", "line": 5, "range": {"start": 2, "end": 9}}) == (2, 9)
        assert model_eval.finding_span({"file": "a", "line": 5}) == (5, 5)
        assert model_eval.finding_span({"file": "a"}) is None
        assert model_eval.finding_span({"file": "a", "range": {"start": "x", "end": 3}}) is None


# ---------------------------------------------------------------------------
# Step 2: hand-built results tree with exact expected numbers
# ---------------------------------------------------------------------------


def write_run(
    out: Path,
    tag: str,
    kind: str,
    target: str,
    run_n: int,
    status: str = "ok",
    findings: list[dict] | None = None,
    wall: float = 10.0,
    model: str = "m/1",
) -> None:
    base = out / tag / ("cases" if kind == "case" else "real") / target
    base.mkdir(parents=True, exist_ok=True)
    (base / f"run{run_n}.json").write_text(json.dumps({"findings": findings or []}), encoding="utf-8")
    meta = {
        "env": {},
        "exit_code": 0 if status == "ok" else 1,
        "findings_count": len(findings or []),
        "hunks_total": 1,
        "kind": kind,
        "model": model,
        "run": run_n,
        "status": status,
        "stet_version": "stet version fake",
        "target": target,
        "wall_seconds": wall,
    }
    (base / f"run{run_n}.meta.json").write_text(json.dumps(meta), encoding="utf-8")


def build_benchmark_cases(tmp_path: Path) -> Path:
    """Two buggy cases (2 bugs, 1 bug) + one clean case."""
    cases = tmp_path / "cases"
    make_case(
        cases,
        case_id="case-a",
        bugs=[
            {"file": "internal/page/page.go", "lines": [4, 4], "category": "correctness", "description": "b0"},
            {"file": "internal/page/page.go", "lines": [8, 8], "category": "performance", "description": "b1"},
        ],
    )
    make_case(
        cases,
        case_id="case-b",
        bugs=[{"file": "cmd/main.go", "lines": [3, 3], "category": "correctness", "description": "b0"}],
    )
    # cmd/main.go must exist in after/ for validation
    (cases / "case-b" / "after" / "cmd").mkdir(parents=True, exist_ok=True)
    (cases / "case-b" / "after" / "cmd" / "main.go.txt").write_text("a\nb\nc\nd\ne\nf\n", encoding="utf-8")
    (cases / "case-b" / "before" / "cmd").mkdir(parents=True, exist_ok=True)
    (cases / "case-b" / "before" / "cmd" / "main.go.txt").write_text("a\nb\nc\n", encoding="utf-8")
    make_case(cases, case_id="case-c", clean=True)
    return cases


def build_benchmark_results(tmp_path: Path) -> Path:
    """Model m__1: exact recall/unexpected/jaccard numbers asserted in tests."""
    out = tmp_path / "results"
    a = "internal/page/page.go"
    # case-a: bug0 caught in runs 1-3, bug1 only in run 1
    write_run(out, "m__1", "case", "case-a", 1, findings=[f_at(a, 4), f_at(a, 8)], wall=10.0)
    write_run(out, "m__1", "case", "case-a", 2, findings=[f_at(a, 4)], wall=20.0)
    write_run(out, "m__1", "case", "case-a", 3, findings=[f_at(a, 4)], wall=30.0)
    # case-b: caught in runs 1 and 3; run 2 has an off-file finding (unexpected warning)
    write_run(out, "m__1", "case", "case-b", 1, findings=[f_at("cmd/main.go", 3)], wall=5.0)
    write_run(out, "m__1", "case", "case-b", 2, findings=[f_at("other/thing.go", 3, severity="warning")], wall=5.0)
    write_run(out, "m__1", "case", "case-b", 3, findings=[f_at("cmd/main.go", 3)], wall=5.0)
    # clean case: one unexpected error finding in run 1
    write_run(out, "m__1", "case", "case-c", 1, findings=[f_at("internal/page/page.go", 1)], wall=1.0)
    write_run(out, "m__1", "case", "case-c", 2, wall=1.0)
    write_run(out, "m__1", "case", "case-c", 3, wall=1.0)
    return out


class TestScoreMetrics:
    @pytest.fixture
    def summary(self, tmp_path: Path) -> dict:
        cases = build_benchmark_cases(tmp_path)
        results = build_benchmark_results(tmp_path)
        cases_by_id = {case.case_id: case for case in model_eval.load_cases(cases)}
        models = model_eval.load_results(results)
        return model_eval.build_summary(cases_by_id, models, window=3, labels_by_anon=None, blind_key=None)

    def test_recall_mean_exact(self, summary: dict) -> None:
        # Runs: 3/3, 1/3, 2/3 -> mean 2/3
        assert summary["m__1"]["recall_mean"] == 0.6667

    def test_caught_lists(self, summary: dict) -> None:
        metrics = summary["m__1"]
        assert metrics["bugs_caught_all_runs"] == []
        assert metrics["bugs_caught_some_runs"] == ["case-a", "case-b"]
        assert metrics["bugs_missed_all_runs"] == []

    def test_recall_by_category_exact(self, summary: dict) -> None:
        by_category = summary["m__1"]["recall_by_category"]
        assert by_category["correctness"] == {"caught_mean": 0.8333, "total": 2}
        assert by_category["performance"] == {"caught_mean": 0.3333, "total": 1}

    def test_unexpected_exact(self, summary: dict) -> None:
        unexpected = summary["m__1"]["unexpected"]
        assert unexpected["total"] == 2
        assert unexpected["mean_per_run"] == 0.2222
        assert unexpected["by_severity"] == {"error": 1, "info": 0, "nitpick": 0, "warning": 1}
        assert unexpected["on_clean_cases"] == 1

    def test_repeatability_exact(self, summary: dict) -> None:
        metrics = summary["m__1"]
        # case-a: 0.5/0.5/1.0 -> 2/3; case-b: 0/1/0 -> 1/3; case-c: 0/0/1.0 (two
        # empty runs agree) -> 1/3; mean = 4/9
        assert metrics["repeatability_jaccard_mean"] == 0.4444
        assert metrics["repeatability_cases_excluded"] == 0

    def test_reliability_and_seconds(self, summary: dict) -> None:
        metrics = summary["m__1"]
        assert metrics["reliability"] == {"failed": 0, "ok": 9, "timeout": 0, "zero_hunks": 0}
        assert metrics["seconds_per_case_mean"] == 8.6667
        assert metrics["seconds_per_real_ref_mean"] == 0.0

    def test_failed_runs_count_in_reliability_and_missed(self, tmp_path: Path) -> None:
        cases = build_benchmark_cases(tmp_path)
        results = build_benchmark_results(tmp_path)
        write_run(results, "z__2", "case", "case-a", 1, findings=[])
        write_run(results, "z__2", "case", "case-b", 1, findings=[])
        write_run(results, "z__2", "case", "case-b", 2, status="failed")
        write_run(results, "z__2", "real", "abc123", 1, wall=60.0)
        cases_by_id = {case.case_id: case for case in model_eval.load_cases(cases)}
        models = model_eval.load_results(results)
        summary = model_eval.build_summary(cases_by_id, models, window=3, labels_by_anon=None, blind_key=None)
        metrics = summary["z__2"]
        assert metrics["recall_mean"] == 0.0
        assert metrics["bugs_missed_all_runs"] == ["case-a", "case-b"]
        assert metrics["reliability"] == {"failed": 1, "ok": 3, "timeout": 0, "zero_hunks": 0}
        assert metrics["seconds_per_real_ref_mean"] == 60.0
        # Single ok run per case -> every case excluded from repeatability.
        assert metrics["repeatability_cases_excluded"] == 3
        assert metrics["repeatability_jaccard_mean"] == 0.0

    def test_jaccard_identical_disjoint_and_excluded(self, tmp_path: Path) -> None:
        cases = build_benchmark_cases(tmp_path)
        results = tmp_path / "results"
        a = "internal/page/page.go"
        # identical pair -> 1.0
        write_run(results, "j__1", "case", "case-a", 1, findings=[f_at(a, 4)])
        write_run(results, "j__1", "case", "case-a", 2, findings=[f_at(a, 4)])
        # disjoint pair -> 0.0 (different files)
        write_run(results, "j__1", "case", "case-b", 1, findings=[f_at("cmd/main.go", 3)])
        write_run(results, "j__1", "case", "case-b", 2, findings=[f_at("other/thing.go", 3)])
        cases_by_id = {case.case_id: case for case in model_eval.load_cases(cases)}
        models = model_eval.load_results(results)
        summary = model_eval.build_summary(cases_by_id, models, window=3, labels_by_anon=None, blind_key=None)
        metrics = summary["j__1"]
        # mean over the two cases: (1.0 + 0.0) / 2; single-run case-c excluded
        assert metrics["repeatability_jaccard_mean"] == 0.5
        assert metrics["repeatability_cases_excluded"] == 1

    def test_empty_results_give_zeroed_metrics(self, tmp_path: Path) -> None:
        cases = build_benchmark_cases(tmp_path)
        results = tmp_path / "results"
        results.mkdir()
        (results / "empty__tag").mkdir()
        cases_by_id = {case.case_id: case for case in model_eval.load_cases(cases)}
        summary = model_eval.build_summary(
            cases_by_id, model_eval.load_results(results), window=3, labels_by_anon=None, blind_key=None
        )
        metrics = summary["empty__tag"]
        assert metrics["recall_mean"] == 0.0
        assert metrics["reliability"] == {"failed": 0, "ok": 0, "timeout": 0, "zero_hunks": 0}
        assert metrics["unexpected"] == {
            "by_severity": {"error": 0, "info": 0, "nitpick": 0, "warning": 0},
            "mean_per_run": 0.0,
            "on_clean_cases": 0,
            "total": 0,
        }


class TestScoreCommand:
    def test_deterministic_bytes_across_invocations(self, tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
        build_benchmark_cases(tmp_path)
        build_benchmark_results(tmp_path)
        args = argparse.Namespace(
            cases=tmp_path / "cases",
            results=tmp_path / "results",
            labels=None,
            blind_key=None,
            out=None,
            check=None,
            window=3,
        )
        assert model_eval.cmd_score(args) == model_eval.EXIT_OK
        first = capsys.readouterr().out
        assert model_eval.cmd_score(args) == model_eval.EXIT_OK
        second = capsys.readouterr().out
        assert first == second
        assert first.endswith("\n")
        parsed = json.loads(first)
        assert list(parsed) == ["m__1"]

    def test_out_writes_same_bytes(self, tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
        build_benchmark_cases(tmp_path)
        build_benchmark_results(tmp_path)
        out_file = tmp_path / "summary.json"
        args = argparse.Namespace(
            cases=tmp_path / "cases",
            results=tmp_path / "results",
            labels=None,
            blind_key=None,
            out=out_file,
            check=None,
            window=3,
        )
        assert model_eval.cmd_score(args) == model_eval.EXIT_OK
        assert out_file.read_text() == capsys.readouterr().out

    def test_check_equal_exits_zero(self, tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
        build_benchmark_cases(tmp_path)
        build_benchmark_results(tmp_path)
        summary_file = tmp_path / "expected-summary.json"
        score_args = argparse.Namespace(
            cases=tmp_path / "cases",
            results=tmp_path / "results",
            labels=None,
            blind_key=None,
            out=None,
            check=None,
            window=3,
        )
        assert model_eval.cmd_score(score_args) == model_eval.EXIT_OK
        summary_file.write_text(capsys.readouterr().out, encoding="utf-8")
        check_args = argparse.Namespace(
            cases=tmp_path / "cases",
            results=tmp_path / "results",
            labels=None,
            blind_key=None,
            out=None,
            check=summary_file,
            window=3,
        )
        assert model_eval.cmd_score(check_args) == model_eval.EXIT_OK
        assert "check ok" in capsys.readouterr().out

    def test_check_mismatch_exits_2_and_names_keys(self, tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
        build_benchmark_cases(tmp_path)
        build_benchmark_results(tmp_path)
        summary_file = tmp_path / "expected-summary.json"
        score_args = argparse.Namespace(
            cases=tmp_path / "cases",
            results=tmp_path / "results",
            labels=None,
            blind_key=None,
            out=None,
            check=None,
            window=3,
        )
        assert model_eval.cmd_score(score_args) == model_eval.EXIT_OK
        summary = json.loads(capsys.readouterr().out)
        summary["recall_mean"] = 9.99
        summary["extra_key"] = 1
        summary_file.write_text(json.dumps(summary), encoding="utf-8")
        check_args = argparse.Namespace(
            cases=tmp_path / "cases",
            results=tmp_path / "results",
            labels=None,
            blind_key=None,
            out=None,
            check=summary_file,
            window=3,
        )
        assert model_eval.cmd_score(check_args) == model_eval.EXIT_CHECK_MISMATCH
        out_text = capsys.readouterr().out
        assert "check mismatch" in out_text
        assert "recall_mean" in out_text
        assert "extra_key" in out_text

    def test_labels_without_blind_key_is_error(self, tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
        build_benchmark_cases(tmp_path)
        build_benchmark_results(tmp_path)
        args = argparse.Namespace(
            cases=tmp_path / "cases",
            results=tmp_path / "results",
            labels=tmp_path / "labels.tsv",
            blind_key=None,
            out=None,
            check=None,
            window=3,
        )
        assert model_eval.cmd_score(args) == model_eval.EXIT_ERROR
        assert "--labels and --blind-key" in capsys.readouterr().err

    def test_missing_results_dir_exits_1(self, tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
        build_benchmark_cases(tmp_path)
        args = argparse.Namespace(
            cases=tmp_path / "cases",
            results=tmp_path / "nope",
            labels=None,
            blind_key=None,
            out=None,
            check=None,
            window=3,
        )
        assert model_eval.cmd_score(args) == model_eval.EXIT_ERROR

    def test_run_json_without_meta_exits_1(self, tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
        build_benchmark_cases(tmp_path)
        results = build_benchmark_results(tmp_path)
        (results / "m__1" / "cases" / "case-a" / "run1.meta.json").unlink()
        args = argparse.Namespace(
            cases=tmp_path / "cases",
            results=results,
            labels=None,
            blind_key=None,
            out=None,
            check=None,
            window=3,
        )
        assert model_eval.cmd_score(args) == model_eval.EXIT_ERROR
        assert "run1.meta.json" in capsys.readouterr().err


# ---------------------------------------------------------------------------
# Step 2: export-blind and label ingestion
# ---------------------------------------------------------------------------


def build_real_results(tmp_path: Path) -> Path:
    out = tmp_path / "results"
    write_run(
        out, "alpha__m", "real", "aaa111", 1,
        findings=[f_at("a.go", 4), f_at("b.go", 9, severity="warning")], model="alpha/m",
    )
    write_run(out, "alpha__m", "real", "bbb222", 1, findings=[f_at("c.go", 2)], model="alpha/m")
    write_run(out, "beta__m", "real", "aaa111", 1, findings=[f_at("d.go", 7)], model="beta/m")
    write_run(out, "beta__m", "real", "aaa111", 2, findings=[f_at("e.go", 1, severity="info")], model="beta/m")
    return out


class TestExportBlind:
    def make_args(self, tmp_path: Path, seed: int = model_eval.DEFAULT_BLIND_SEED) -> argparse.Namespace:
        return argparse.Namespace(
            results=tmp_path / "results",
            out=tmp_path / "blind.tsv",
            key=tmp_path / "blind-key.json",
            seed=seed,
        )

    def test_deterministic_for_seed_and_hides_model(self, tmp_path: Path, capsys: pytest.CaptureFixture[str]) -> None:
        build_real_results(tmp_path)
        assert model_eval.cmd_export_blind(self.make_args(tmp_path)) == model_eval.EXIT_OK
        first_tsv = (tmp_path / "blind.tsv").read_text()
        first_key = (tmp_path / "blind-key.json").read_text()
        assert model_eval.cmd_export_blind(self.make_args(tmp_path)) == model_eval.EXIT_OK
        assert (tmp_path / "blind.tsv").read_text() == first_tsv
        assert (tmp_path / "blind-key.json").read_text() == first_key

        # No model identity anywhere in the TSV.
        for fragment in ("alpha", "beta", "alpha__m", "beta__m"):
            assert fragment not in first_tsv
        # Header + 5 findings, 10 columns, empty label/notes.
        lines = first_tsv.rstrip("\n").split("\n")
        assert lines[0].split("\t") == list(model_eval.BLIND_TSV_COLUMNS)
        assert len(lines) == 6
        for line in lines[1:]:
            cols = line.split("\t")
            assert len(cols) == 10
            assert cols[8] == "" and cols[9] == ""
        # Anon ids are F001..F005; the key maps back to models.
        key = json.loads(first_key)
        assert sorted(key) == [f"F{i:03d}" for i in range(1, 6)]
        model_tags = {entry["model_tag"] for entry in key.values()}
        assert model_tags == {"alpha__m", "beta__m"}
        assert "exported 5 findings" in capsys.readouterr().out

    def test_different_seed_changes_order(self, tmp_path: Path) -> None:
        build_real_results(tmp_path)
        model_eval.cmd_export_blind(self.make_args(tmp_path, seed=1))
        tsv_seed_1 = (tmp_path / "blind.tsv").read_text()
        model_eval.cmd_export_blind(self.make_args(tmp_path, seed=2))
        tsv_seed_2 = (tmp_path / "blind.tsv").read_text()
        assert tsv_seed_1 != tsv_seed_2

    def test_empty_results_exports_header_only(self, tmp_path: Path) -> None:
        (tmp_path / "results").mkdir()
        assert model_eval.cmd_export_blind(self.make_args(tmp_path)) == model_eval.EXIT_OK
        tsv = (tmp_path / "blind.tsv").read_text()
        assert tsv.rstrip("\n").split("\t") == list(model_eval.BLIND_TSV_COLUMNS)
        assert json.loads((tmp_path / "blind-key.json").read_text()) == {}

    def test_missing_results_dir_exits_1(self, tmp_path: Path) -> None:
        args = self.make_args(tmp_path)
        args.results = tmp_path / "nope"
        assert model_eval.cmd_export_blind(args) == model_eval.EXIT_ERROR


class TestLabels:
    @pytest.fixture
    def labeled(self, tmp_path: Path) -> tuple[dict, dict, Path]:
        build_benchmark_cases(tmp_path)
        build_real_results(tmp_path)
        tsv_path = tmp_path / "blind.tsv"
        key_path = tmp_path / "blind-key.json"
        args = argparse.Namespace(results=tmp_path / "results", out=tsv_path, key=key_path, seed=42)
        assert model_eval.cmd_export_blind(args) == model_eval.EXIT_OK
        key = json.loads(key_path.read_text())
        lines = tsv_path.read_text().rstrip("\n").split("\n")
        # Label the 5 findings: two actionable, one false_positive, one out_of_scope, one wrong_suggestion.
        chosen_labels = ["actionable", "false_positive", "actionable", "out_of_scope", "wrong_suggestion"]
        relabeled = [lines[0]]
        for line, label in zip(lines[1:], chosen_labels):
            cols = line.split("\t")
            cols[8] = label
            relabeled.append("\t".join(cols))
        tsv_path.write_text("\n".join(relabeled) + "\n", encoding="utf-8")
        return key, {"rows": relabeled}, tsv_path

    def test_load_labels_against_key(self, labeled) -> None:
        key, _rows, tsv_path = labeled
        tmp_path = tsv_path.parent
        labels = model_eval.load_labels(tsv_path, tmp_path / "blind-key.json")
        assert len(labels) == 5
        assert set(labels.values()) == {"actionable", "false_positive", "out_of_scope", "wrong_suggestion"}

    def test_real_diff_per_model(self, tmp_path: Path, labeled, capsys: pytest.CaptureFixture[str]) -> None:
        key, _rows, tsv_path = labeled
        key_path = tmp_path / "blind-key.json"
        labels_by_anon = model_eval.load_labels(tsv_path, key_path)
        cases_by_id = {case.case_id: case for case in model_eval.load_cases(tmp_path / "cases")}
        models = model_eval.load_results(tmp_path / "results")
        summary = model_eval.build_summary(cases_by_id, models, 3, labels_by_anon, json.loads(key_path.read_text()))
        alpha = summary["alpha__m"]["real_diff"]
        beta = summary["beta__m"]["real_diff"]
        assert alpha["labeled"] + beta["labeled"] == 5
        for metrics in (alpha, beta):
            assert set(metrics["labels"]) == set(model_eval.VALID_LABELS)
            assert metrics["actionable_rate"] == round(metrics["labels"]["actionable"] / metrics["labeled"], 4)
        # All 5 labeled findings are split across exactly the two models.
        assert sorted(
            [summary["alpha__m"]["real_diff"]["labeled"], summary["beta__m"]["real_diff"]["labeled"]]
        ) == [2, 3]

    def _rewrite_labels(self, tsv_path: Path, row_index: int, column: int, value: str) -> None:
        lines = tsv_path.read_text().rstrip("\n").split("\n")
        cols = lines[row_index].split("\t")
        cols[column] = value
        lines[row_index] = "\t".join(cols)
        tsv_path.write_text("\n".join(lines) + "\n", encoding="utf-8")

    def test_unknown_label_names_row(self, tmp_path: Path, labeled, capsys: pytest.CaptureFixture[str]) -> None:
        _key, _rows, tsv_path = labeled
        self._rewrite_labels(tsv_path, 2, 8, "maybe")  # file line 3
        with pytest.raises(model_eval.HarnessError, match=r"labels row 3.*'maybe'"):
            model_eval.load_labels(tsv_path, tmp_path / "blind-key.json")

    def test_unknown_anon_id_names_row(self, tmp_path: Path, labeled, capsys: pytest.CaptureFixture[str]) -> None:
        _key, _rows, tsv_path = labeled
        self._rewrite_labels(tsv_path, 3, 0, "F999")  # file line 4
        with pytest.raises(model_eval.HarnessError, match=r"labels row 4.*F999"):
            model_eval.load_labels(tsv_path, tmp_path / "blind-key.json")

    def test_unlabeled_rows_are_skipped(self, tmp_path: Path, labeled) -> None:
        _key, _rows, tsv_path = labeled
        self._rewrite_labels(tsv_path, 1, 8, "")
        labels = model_eval.load_labels(tsv_path, tmp_path / "blind-key.json")
        assert len(labels) == 4


# ---------------------------------------------------------------------------
# Step 3: --dry-run integration test (real stet, skipped when unavailable)
# ---------------------------------------------------------------------------


SCRIPT_PATH = Path(__file__).resolve().parents[1] / "model_eval.py"


@pytest.mark.skipif(shutil.which("stet") is None, reason="stet not on PATH")
@pytest.mark.skipif(shutil.which("git") is None, reason="git not on PATH")
class TestDryRunIntegration:
    def test_run_then_score_on_one_case_fixture(self, tmp_path: Path) -> None:
        make_case(tmp_path / "cases", case_id="go-integration")
        out_dir = tmp_path / "out"
        run_proc = subprocess.run(
            [
                sys.executable,
                str(SCRIPT_PATH),
                "run",
                "--dry-run",
                "--runs",
                "1",
                "--cases",
                str(tmp_path / "cases"),
                "--out",
                str(out_dir),
            ],
            capture_output=True,
            text=True,
            timeout=300,
        )
        assert run_proc.returncode == model_eval.EXIT_OK, run_proc.stderr
        meta_path = out_dir / "dry-run" / "cases" / "go-integration" / "run1.meta.json"
        assert meta_path.is_file()
        meta = json.loads(meta_path.read_text())
        assert meta["status"] == "ok"
        assert meta["hunks_total"] >= 1
        assert meta["kind"] == "case"
        assert (out_dir / "dry-run" / "cases" / "go-integration" / "run1.json").is_file()

        score_proc = subprocess.run(
            [
                sys.executable,
                str(SCRIPT_PATH),
                "score",
                "--cases",
                str(tmp_path / "cases"),
                "--results",
                str(out_dir),
            ],
            capture_output=True,
            text=True,
            timeout=120,
        )
        assert score_proc.returncode == model_eval.EXIT_OK, score_proc.stderr
        summary = json.loads(score_proc.stdout)
        assert "dry-run" in summary
        assert summary["dry-run"]["reliability"]["ok"] == 1


class TestInputValidation:
    @pytest.mark.parametrize(
        "kwargs",
        [
            {"real_refs": ["--upload-pack=evil"]},
            {"base_url": "file:///etc/passwd"},
            {"base_url": "not-a-url"},
        ],
    )
    def test_malicious_options_exits_1(self, tmp_path: Path, fake_tools: FakeSimple, kwargs: dict) -> None:
        make_case(tmp_path / "cases")
        assert model_eval.cmd_run(make_run_args(tmp_path, **kwargs), clock=FakeClock()) == model_eval.EXIT_ERROR

    def test_valid_refs_and_urls_pass(self, tmp_path: Path, monkeypatch: pytest.MonkeyPatch) -> None:
        make_case(tmp_path / "cases")
        monkeypatch.setattr(model_eval, "run_simple", FakeSimple())
        monkeypatch.setattr(model_eval, "run_stream", FakeRunner())
        monkeypatch.setattr(model_eval, "which", lambda name: f"/fake-bin/{name}")
        args = make_run_args(tmp_path, real_refs=["abc123", "deadbeef"], base_url="https://llm.local/v1")
        assert model_eval.cmd_run(args, clock=FakeClock()) == model_eval.EXIT_OK
