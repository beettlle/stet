"""Read .review/history.jsonl and rotated archives; extract dismissal records.

Mirrors Go ``history.ReadRecords`` semantics (``cli/internal/history/append.go``).
"""

from __future__ import annotations

import gzip
import json
from collections.abc import Iterator
from dataclasses import dataclass
from pathlib import Path
from typing import Any

VALID_REASONS = frozenset(
    {
        "false_positive",
        "already_correct",
        "wrong_suggestion",
        "out_of_scope",
    }
)

HISTORY_FILENAME = "history.jsonl"
HISTORY_GZ_PREFIX = "history.jsonl."
HISTORY_GZ_SUFFIX = ".gz"


class HistoryLoadError(Exception):
    """Raised when history files cannot be read or parsed."""


@dataclass(frozen=True)
class DismissalRecord:
    """One dismissed finding joined with review-output metadata."""

    finding_id: str
    reason: str
    prompt_context: str
    category: str
    message: str
    diff_ref: str = ""


def valid_reason(reason: str) -> bool:
    """Return True when *reason* is one of the allowed dismissal constants."""
    return reason in VALID_REASONS


def _parse_archive_number(mid: str) -> int | None:
    try:
        number = int(mid)
    except ValueError:
        return None
    if number < 1 or number > 2**31 - 1:
        return None
    return number


def _list_archive_paths(state_dir: Path) -> list[Path]:
    archives: list[tuple[int, Path]] = []
    for entry in state_dir.iterdir():
        if not entry.is_file():
            continue
        name = entry.name
        if not name.startswith(HISTORY_GZ_PREFIX) or not name.endswith(HISTORY_GZ_SUFFIX):
            continue
        mid = name[len(HISTORY_GZ_PREFIX) : len(name) - len(HISTORY_GZ_SUFFIX)]
        number = _parse_archive_number(mid)
        if number is None:
            continue
        archives.append((number, entry))
    archives.sort(key=lambda item: item[0])
    return [path for _, path in archives]


def _read_lines_from_reader(reader: Any) -> list[str]:
    lines: list[str] = []
    for line in reader:
        if isinstance(line, bytes):
            line = line.decode("utf-8")
        lines.append(line if line.endswith("\n") else f"{line}\n")
    return lines


def _read_lines_from_path(path: Path) -> list[str]:
    with path.open(encoding="utf-8") as handle:
        return _read_lines_from_reader(handle)


def _read_lines_from_gzip(path: Path) -> list[str]:
    with gzip.open(path, mode="rt", encoding="utf-8") as handle:
        return _read_lines_from_reader(handle)


def _parse_record_lines(lines: list[str]) -> list[dict[str, Any]]:
    records: list[dict[str, Any]] = []
    for line in lines:
        stripped = line.strip()
        if not stripped:
            continue
        try:
            payload = json.loads(stripped)
        except json.JSONDecodeError as exc:
            raise HistoryLoadError(f"invalid history line: {exc}") from exc
        if not isinstance(payload, dict):
            raise HistoryLoadError("invalid history line: expected JSON object")
        records.append(payload)
    return records


def read_records(state_dir: str | Path) -> list[dict[str, Any]]:
    """Read history records oldest-first: archives by N, then active file."""
    directory = Path(state_dir)
    if not directory.is_dir():
        raise HistoryLoadError(f"Could not read history directory: {directory}")

    records: list[dict[str, Any]] = []
    for archive_path in _list_archive_paths(directory):
        try:
            lines = _read_lines_from_gzip(archive_path)
        except OSError as exc:
            raise HistoryLoadError(f"Could not read history archive: {archive_path}") from exc
        records.extend(_parse_record_lines(lines))

    active_path = directory / HISTORY_FILENAME
    if active_path.exists():
        try:
            lines = _read_lines_from_path(active_path)
        except OSError as exc:
            raise HistoryLoadError(f"Could not read history file: {active_path}") from exc
        records.extend(_parse_record_lines(lines))

    return records


def _findings_by_id(review_output: list[Any]) -> dict[str, dict[str, Any]]:
    by_id: dict[str, dict[str, Any]] = {}
    for finding in review_output:
        if not isinstance(finding, dict):
            continue
        finding_id = finding.get("id")
        if finding_id:
            by_id[str(finding_id)] = finding
    return by_id


def _dismissal_entry(
    *,
    finding_id: str,
    reason: str,
    prompt_context: str,
    finding: dict[str, Any],
    diff_ref: str,
) -> DismissalRecord:
    return DismissalRecord(
        finding_id=finding_id,
        reason=reason,
        prompt_context=prompt_context,
        category=str(finding.get("category", "")),
        message=str(finding.get("message", "")),
        diff_ref=diff_ref,
    )


def iter_dismissals(records: list[dict[str, Any]]) -> Iterator[DismissalRecord]:
    """Yield dismissal records joined with finding category and message."""
    for record in records:
        review_output = record.get("review_output") or []
        if not isinstance(review_output, list):
            review_output = []
        user_action = record.get("user_action") or {}
        if not isinstance(user_action, dict):
            user_action = {}
        dismissals = user_action.get("dismissals") or []
        if not isinstance(dismissals, list):
            dismissals = []
        dismissed_ids = user_action.get("dismissed_ids") or []
        if not isinstance(dismissed_ids, list):
            dismissed_ids = []

        diff_ref = str(record.get("diff_ref", ""))
        by_id = _findings_by_id(review_output)
        extracted_with_reason: set[str] = set()
        dismissal_by_id: dict[str, dict[str, Any]] = {}

        for dismissal in dismissals:
            if not isinstance(dismissal, dict):
                continue
            finding_id = str(dismissal.get("finding_id", ""))
            if not finding_id:
                continue
            dismissal_by_id[finding_id] = dismissal
            reason = str(dismissal.get("reason", ""))
            if not reason or not valid_reason(reason):
                continue
            finding = by_id.get(finding_id)
            if finding is None:
                continue
            extracted_with_reason.add(finding_id)
            yield _dismissal_entry(
                finding_id=finding_id,
                reason=reason,
                prompt_context=str(dismissal.get("prompt_context", "")),
                finding=finding,
                diff_ref=diff_ref,
            )

        for finding_id in dismissed_ids:
            finding_id = str(finding_id)
            if not finding_id or finding_id in extracted_with_reason:
                continue
            finding = by_id.get(finding_id)
            if finding is None:
                continue
            dismissal = dismissal_by_id.get(finding_id, {})
            reason = str(dismissal.get("reason", ""))
            if reason and not valid_reason(reason):
                reason = ""
            yield _dismissal_entry(
                finding_id=finding_id,
                reason=reason if valid_reason(reason) else "",
                prompt_context=str(dismissal.get("prompt_context", "")),
                finding=finding,
                diff_ref=diff_ref,
            )


def extract_dismissals(records: list[dict[str, Any]]) -> list[DismissalRecord]:
    """Return all dismissals from parsed history records."""
    return list(iter_dismissals(records))


def load_dismissals(state_dir: str | Path) -> list[DismissalRecord]:
    """Read history from *state_dir* and extract dismissal records."""
    return extract_dismissals(read_records(state_dir))
