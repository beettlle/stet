#!/usr/bin/env python3
"""Stet optimizer sidecar entrypoint (SDRE Phase 0).

Invoked by the Go CLI via STET_OPTIMIZER_SCRIPT with STET_STATE_DIR set.
"""

from __future__ import annotations

import os
import sys
import tempfile
from pathlib import Path

from scripts.optimizer.history_loader import (
    HistoryLoadError,
    extract_dismissals,
    read_records,
)
from scripts.optimizer.metrics import print_summary
from scripts.optimizer.prompt_validator import validate_prompt
from scripts.optimizer.rule_based import build_optimized_prompt

EXIT_SUCCESS = 0
EXIT_HISTORY_ERROR = 1
EXIT_VALIDATION_ERROR = 2

OUTPUT_FILENAME = "system_prompt_optimized.txt"
NO_DISMISSALS_MESSAGE = "no dismissals in history; skipping optimized prompt write"


def _atomic_write_text(path: Path, content: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    fd, temp_name = tempfile.mkstemp(
        prefix=f".{path.name}.",
        suffix=".tmp",
        dir=path.parent,
        text=True,
    )
    temp_path = Path(temp_name)
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as handle:
            handle.write(content)
            handle.flush()
            os.fsync(handle.fileno())
        os.replace(temp_path, path)
    except Exception:
        temp_path.unlink(missing_ok=True)
        raise


def run() -> int:
    """Run optimizer; return process exit code per optimizer-sidecar contract."""
    state_dir = os.environ.get("STET_STATE_DIR", "").strip()
    if not state_dir:
        print("error: STET_STATE_DIR is not set", file=sys.stderr)
        return EXIT_HISTORY_ERROR

    try:
        records = read_records(state_dir)
    except HistoryLoadError as exc:
        print(f"error: {exc}", file=sys.stderr)
        return EXIT_HISTORY_ERROR

    dismissals = extract_dismissals(records)
    print_summary(len(records), dismissals)

    if not dismissals:
        print(NO_DISMISSALS_MESSAGE, file=sys.stderr)
        return EXIT_SUCCESS

    prompt = build_optimized_prompt(dismissals)
    validation = validate_prompt(prompt)
    if not validation.ok:
        for violation in validation.violations:
            print(f"validation error: {violation}", file=sys.stderr)
        return EXIT_VALIDATION_ERROR

    output_path = Path(state_dir) / OUTPUT_FILENAME
    try:
        _atomic_write_text(output_path, prompt)
    except OSError as exc:
        print(f"error: could not write {output_path}: {exc}", file=sys.stderr)
        return EXIT_HISTORY_ERROR

    return EXIT_SUCCESS


def main() -> None:
    sys.exit(run())


if __name__ == "__main__":
    main()
