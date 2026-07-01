"""Tests for scripts.optimizer.history_loader."""

from __future__ import annotations

import gzip
import json
from pathlib import Path

import pytest

from scripts.optimizer.history_loader import (
    DismissalRecord,
    HistoryLoadError,
    extract_dismissals,
    load_dismissals,
    read_records,
    valid_reason,
)

FIXTURES_DIR = Path(__file__).resolve().parent / "fixtures"
HISTORY_FIXTURE = FIXTURES_DIR / "history.jsonl"


def _write_jsonl(path: Path, records: list[dict]) -> None:
    lines = [json.dumps(record) for record in records]
    path.write_text("\n".join(lines) + ("\n" if lines else ""), encoding="utf-8")


def _write_gzip_jsonl(path: Path, records: list[dict]) -> None:
    payload = "\n".join(json.dumps(record) for record in records) + "\n"
    with gzip.open(path, "wt", encoding="utf-8") as handle:
        handle.write(payload)


def test_valid_reason_constants() -> None:
    assert valid_reason("false_positive")
    assert valid_reason("already_correct")
    assert valid_reason("wrong_suggestion")
    assert valid_reason("out_of_scope")
    assert not valid_reason("invalid")
    assert not valid_reason("")


def test_load_fixture_extracts_all_reason_types() -> None:
    state_dir = FIXTURES_DIR
    dismissals = load_dismissals(state_dir)

    assert len(dismissals) == 4
    reasons = {item.reason for item in dismissals}
    assert reasons == {
        "false_positive",
        "already_correct",
        "wrong_suggestion",
        "out_of_scope",
    }

    by_id = {item.finding_id: item for item in dismissals}
    assert by_id["f-fp"].category == "bug"
    assert by_id["f-fp"].message == "Possible nil dereference"
    assert by_id["f-fp"].prompt_context == "@@ hunk false_positive"
    assert by_id["f-ac"].category == "style"
    assert by_id["f-ws"].category == "correctness"
    assert by_id["f-oos"].category == "documentation"


def test_read_records_from_fixture_file(tmp_path: Path) -> None:
    active = tmp_path / "history.jsonl"
    active.write_text(HISTORY_FIXTURE.read_text(encoding="utf-8"), encoding="utf-8")

    records = read_records(tmp_path)
    assert len(records) == 1
    assert records[0]["diff_ref"] == "abc123"


def test_empty_history_directory_returns_zero_dismissals(tmp_path: Path) -> None:
    dismissals = load_dismissals(tmp_path)
    assert dismissals == []


def test_empty_history_file_returns_zero_dismissals(tmp_path: Path) -> None:
    (tmp_path / "history.jsonl").write_text("", encoding="utf-8")
    dismissals = load_dismissals(tmp_path)
    assert dismissals == []


def test_invalid_json_line_raises(tmp_path: Path) -> None:
    (tmp_path / "history.jsonl").write_text("{not-json}\n", encoding="utf-8")

    with pytest.raises(HistoryLoadError, match="invalid history line"):
        read_records(tmp_path)


def test_archive_then_active_chronological_order(tmp_path: Path) -> None:
    archive_record = {
        "diff_ref": "archived",
        "review_output": [
            {
                "id": "f-arch",
                "file": "pkg/arch.go",
                "severity": "warning",
                "category": "bug",
                "confidence": 0.5,
                "message": "Archived dismissal",
            }
        ],
        "user_action": {
            "dismissals": [
                {
                    "finding_id": "f-arch",
                    "reason": "false_positive",
                }
            ]
        },
    }
    active_record = {
        "diff_ref": "active",
        "review_output": [
            {
                "id": "f-active",
                "file": "pkg/active.go",
                "severity": "warning",
                "category": "bug",
                "confidence": 0.5,
                "message": "Active dismissal",
            }
        ],
        "user_action": {
            "dismissals": [
                {
                    "finding_id": "f-active",
                    "reason": "wrong_suggestion",
                }
            ]
        },
    }

    _write_gzip_jsonl(tmp_path / "history.jsonl.1.gz", [archive_record])
    _write_gzip_jsonl(tmp_path / "history.jsonl.2.gz", [])
    _write_jsonl(tmp_path / "history.jsonl", [active_record])

    records = read_records(tmp_path)
    assert [record["diff_ref"] for record in records] == ["archived", "active"]

    dismissals = extract_dismissals(records)
    assert [item.finding_id for item in dismissals] == ["f-arch", "f-active"]
    assert dismissals[0] == DismissalRecord(
        finding_id="f-arch",
        reason="false_positive",
        prompt_context="",
        category="bug",
        message="Archived dismissal",
        diff_ref="archived",
    )


def test_dismissed_ids_without_explicit_reason(tmp_path: Path) -> None:
    record = {
        "diff_ref": "ids-only",
        "review_output": [
            {
                "id": "f-only",
                "file": "pkg/only.go",
                "severity": "warning",
                "category": "testing",
                "confidence": 0.5,
                "message": "Dismissed via id list",
            }
        ],
        "user_action": {"dismissed_ids": ["f-only"]},
    }
    _write_jsonl(tmp_path / "history.jsonl", [record])

    dismissals = load_dismissals(tmp_path)
    assert len(dismissals) == 1
    assert dismissals[0].finding_id == "f-only"
    assert dismissals[0].reason == ""
    assert dismissals[0].category == "testing"


def test_invalid_reason_in_dismissals_is_ignored(tmp_path: Path) -> None:
    record = {
        "diff_ref": "invalid-reason",
        "review_output": [
            {
                "id": "f-bad",
                "file": "pkg/bad.go",
                "severity": "warning",
                "category": "bug",
                "confidence": 0.5,
                "message": "Should be ignored",
            }
        ],
        "user_action": {
            "dismissals": [{"finding_id": "f-bad", "reason": "not_a_real_reason"}]
        },
    }
    _write_jsonl(tmp_path / "history.jsonl", [record])

    dismissals = load_dismissals(tmp_path)
    assert dismissals == []
