"""Integration tests for scripts/optimize.py entrypoint."""

from __future__ import annotations

import json
import os
import shutil
import subprocess
import sys
from pathlib import Path
from unittest.mock import patch

import pytest

from scripts.optimizer.prompt_validator import assert_valid_prompt
from scripts.optimize import NO_DISMISSALS_MESSAGE, OUTPUT_FILENAME, run

REPO_ROOT = Path(__file__).resolve().parents[2]
OPTIMIZE_SCRIPT = REPO_ROOT / "scripts" / "optimize.py"
FIXTURES_DIR = Path(__file__).resolve().parent / "fixtures"
HISTORY_FIXTURE = FIXTURES_DIR / "history.jsonl"


def _run_optimize_subprocess(state_dir: Path) -> subprocess.CompletedProcess[str]:
    env = {
        **os.environ,
        "STET_STATE_DIR": str(state_dir),
        "PYTHONPATH": str(REPO_ROOT),
    }
    return subprocess.run(
        [sys.executable, str(OPTIMIZE_SCRIPT)],
        env=env,
        capture_output=True,
        text=True,
        check=False,
    )


def _run_with_state_dir(state_dir: Path) -> int:
    previous = os.environ.get("STET_STATE_DIR")
    os.environ["STET_STATE_DIR"] = str(state_dir)
    try:
        return run()
    finally:
        if previous is None:
            os.environ.pop("STET_STATE_DIR", None)
        else:
            os.environ["STET_STATE_DIR"] = previous


def test_empty_history_file_exits_zero_without_output(tmp_path: Path) -> None:
    (tmp_path / "history.jsonl").write_text("", encoding="utf-8")

    result = _run_optimize_subprocess(tmp_path)

    assert result.returncode == 0
    assert NO_DISMISSALS_MESSAGE in result.stderr
    assert "0 dismissals" in result.stderr
    assert not (tmp_path / OUTPUT_FILENAME).exists()


def test_empty_history_directory_exits_zero_without_output(tmp_path: Path) -> None:
    result = _run_optimize_subprocess(tmp_path)

    assert result.returncode == 0
    assert NO_DISMISSALS_MESSAGE in result.stderr
    assert not (tmp_path / OUTPUT_FILENAME).exists()


def test_missing_state_directory_exits_nonzero(tmp_path: Path) -> None:
    missing = tmp_path / "missing-state"
    result = _run_optimize_subprocess(missing)

    assert result.returncode == 1
    assert "error:" in result.stderr
    assert not (missing / OUTPUT_FILENAME).exists()


def test_missing_stet_state_dir_env_exits_nonzero(monkeypatch: pytest.MonkeyPatch) -> None:
    monkeypatch.delenv("STET_STATE_DIR", raising=False)
    assert run() == 1


def test_malformed_history_exits_nonzero(tmp_path: Path) -> None:
    (tmp_path / "history.jsonl").write_text("{not json}\n", encoding="utf-8")

    result = _run_optimize_subprocess(tmp_path)

    assert result.returncode == 1
    assert "error:" in result.stderr


def test_valid_history_writes_optimized_prompt(tmp_path: Path) -> None:
    shutil.copy(HISTORY_FIXTURE, tmp_path / "history.jsonl")

    result = _run_optimize_subprocess(tmp_path)

    assert result.returncode == 0, result.stderr
    output_path = tmp_path / OUTPUT_FILENAME
    assert output_path.exists()
    prompt = output_path.read_text(encoding="utf-8")
    assert "## Lessons learned" in prompt
    assert "4 dismissals" in result.stderr
    assert "false_positive: 1" in result.stderr
    assert_valid_prompt(prompt)


def test_valid_history_is_deterministic(tmp_path: Path) -> None:
    shutil.copy(HISTORY_FIXTURE, tmp_path / "history.jsonl")

    first = _run_optimize_subprocess(tmp_path)
    first_prompt = (tmp_path / OUTPUT_FILENAME).read_text(encoding="utf-8")

    second = _run_optimize_subprocess(tmp_path)
    second_prompt = (tmp_path / OUTPUT_FILENAME).read_text(encoding="utf-8")

    assert first.returncode == 0
    assert second.returncode == 0
    assert first_prompt == second_prompt


def test_validator_rejection_exits_two(tmp_path: Path) -> None:
    shutil.copy(HISTORY_FIXTURE, tmp_path / "history.jsonl")

    with patch("scripts.optimize.build_optimized_prompt", return_value="invalid prompt"):
        code = _run_with_state_dir(tmp_path)

    assert code == 2
    assert not (tmp_path / OUTPUT_FILENAME).exists()


def test_noop_does_not_overwrite_existing_prompt(tmp_path: Path) -> None:
    output_path = tmp_path / OUTPUT_FILENAME
    output_path.write_text("existing prompt", encoding="utf-8")
    (tmp_path / "history.jsonl").write_text(
        json.dumps({"diff_ref": "x", "review_output": [], "user_action": {}}) + "\n",
        encoding="utf-8",
    )

    code = _run_with_state_dir(tmp_path)

    assert code == 0
    assert output_path.read_text(encoding="utf-8") == "existing prompt"
