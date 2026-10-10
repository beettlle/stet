"""Tests for scripts.model_eval (SP-037 model comparison harness)."""

from __future__ import annotations

import argparse
import json
import shutil
import subprocess
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
        assert env["STET_MAX_COMPLETION_TOKENS"] == "8192"
        assert env["STET_NUM_CTX"] == "32768"
        assert env["STET_CONTEXT_LIMIT"] == "32768"
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
