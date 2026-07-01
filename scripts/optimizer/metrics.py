"""Summary statistics for optimizer runs (stderr)."""

from __future__ import annotations

import sys
from collections import Counter
from typing import TextIO

from scripts.optimizer.history_loader import DismissalRecord, valid_reason

_REASON_ORDER = (
    "false_positive",
    "already_correct",
    "wrong_suggestion",
    "out_of_scope",
)


def format_summary(record_count: int, dismissals: list[DismissalRecord]) -> str:
    """Return human-readable summary stats for stderr."""
    record_label = "record" if record_count == 1 else "records"
    dismissal_label = "dismissal" if len(dismissals) == 1 else "dismissals"
    lines = [
        f"optimizer: {record_count} history {record_label}, {len(dismissals)} {dismissal_label}",
    ]

    counts = Counter(
        dismissal.reason for dismissal in dismissals if valid_reason(dismissal.reason)
    )
    if counts:
        lines.append("dismissals by reason:")
        for reason in _REASON_ORDER:
            if reason in counts:
                lines.append(f"  {reason}: {counts[reason]}")
        for reason, count in sorted(counts.items()):
            if reason not in _REASON_ORDER:
                lines.append(f"  {reason}: {count}")

    return "\n".join(lines)


def print_summary(
    record_count: int,
    dismissals: list[DismissalRecord],
    *,
    stream: TextIO | None = None,
) -> None:
    """Print summary stats to stderr."""
    target = stream if stream is not None else sys.stderr
    print(format_summary(record_count, dismissals), file=target)
