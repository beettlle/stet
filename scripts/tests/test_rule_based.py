"""Unit tests for scripts/optimizer/rule_based.py."""

from __future__ import annotations

from pathlib import Path

from scripts.optimizer.history_loader import DismissalRecord, load_dismissals
from scripts.optimizer.prompt_validator import assert_valid_prompt
from scripts.optimizer.rule_based import (
    DEFAULT_SYSTEM_PROMPT,
    build_optimized_prompt,
    default_prompt_length,
    normalize_message,
    prompt_length_cap,
)

FIXTURES_DIR = Path(__file__).resolve().parent / "fixtures"
DEFAULT_PROMPT_PATH = FIXTURES_DIR / "default_prompt.txt"


def _dismissal(
    *,
    finding_id: str,
    reason: str,
    category: str,
    message: str,
    prompt_context: str = "",
) -> DismissalRecord:
    return DismissalRecord(
        finding_id=finding_id,
        reason=reason,
        prompt_context=prompt_context,
        category=category,
        message=message,
    )


def test_default_prompt_matches_fixture() -> None:
    fixture = DEFAULT_PROMPT_PATH.read_text(encoding="utf-8").rstrip("\n")
    assert DEFAULT_SYSTEM_PROMPT == fixture


def test_empty_dismissals_returns_default_prompt() -> None:
    assert build_optimized_prompt([]) == DEFAULT_SYSTEM_PROMPT


def test_build_is_deterministic() -> None:
    dismissals = load_dismissals(FIXTURES_DIR)
    first = build_optimized_prompt(dismissals)
    second = build_optimized_prompt(dismissals)
    assert first == second


def test_fixture_history_includes_lessons_and_passes_validator() -> None:
    dismissals = load_dismissals(FIXTURES_DIR)
    prompt = build_optimized_prompt(dismissals)

    assert "## Lessons learned" in prompt
    assert prompt.startswith(DEFAULT_SYSTEM_PROMPT)
    assert len(prompt) <= prompt_length_cap()
    assert_valid_prompt(prompt)


def test_reason_mapping_negative_examples() -> None:
    dismissals = [
        _dismissal(
            finding_id="a",
            reason="false_positive",
            category="bug",
            message="Possible nil dereference",
            prompt_context="@@ hunk ctx",
        ),
        _dismissal(
            finding_id="b",
            reason="wrong_suggestion",
            category="correctness",
            message="Wrong return type suggested",
            prompt_context="@@ other ctx",
        ),
    ]
    prompt = build_optimized_prompt(dismissals)

    assert "### Negative examples (dismissed findings)" in prompt
    assert "Do not report issues similar to these dismissed findings." in prompt
    assert "Context (dismissed):" in prompt
    assert "@@ hunk ctx" in prompt
    assert "@@ other ctx" in prompt


def test_reason_mapping_verify_before_reporting() -> None:
    dismissals = [
        _dismissal(
            finding_id="a",
            reason="already_correct",
            category="style",
            message="Unused variable",
        ),
    ]
    prompt = build_optimized_prompt(dismissals)

    assert "### Verify before reporting" in prompt
    assert "Verify carefully before reporting similar issues." in prompt
    assert "Context (dismissed):" not in prompt


def test_reason_mapping_scope_filters() -> None:
    dismissals = [
        _dismissal(
            finding_id="a",
            reason="out_of_scope",
            category="documentation",
            message="Missing doc comment",
        ),
    ]
    prompt = build_optimized_prompt(dismissals)

    assert "### Scope filters" in prompt
    assert "out of scope" in prompt.lower()


def test_aggregates_by_normalized_message() -> None:
    dismissals = [
        _dismissal(
            finding_id="a",
            reason="false_positive",
            category="bug",
            message="Possible  nil   dereference",
        ),
        _dismissal(
            finding_id="b",
            reason="false_positive",
            category="bug",
            message="possible nil dereference",
        ),
    ]
    prompt = build_optimized_prompt(dismissals)

    assert prompt.count("Possible  nil   dereference") == 1
    assert "dismissed 2 times" in prompt


def test_prompt_respects_two_times_length_cap() -> None:
    long_message = "x" * 500
    dismissals = [
        _dismissal(
            finding_id=f"id-{index}",
            reason="false_positive",
            category="bug",
            message=f"{long_message}-{index}",
            prompt_context=f"context-{index}\n" + ("y" * 200),
        )
        for index in range(40)
    ]
    prompt = build_optimized_prompt(dismissals)

    assert len(prompt) <= prompt_length_cap()
    assert_valid_prompt(prompt)


def test_cap_keeps_higher_frequency_patterns() -> None:
    frequent = _dismissal(
        finding_id="freq",
        reason="false_positive",
        category="bug",
        message="FREQUENT_PATTERN",
    )
    rare = _dismissal(
        finding_id="rare",
        reason="false_positive",
        category="bug",
        message="RARE_PATTERN",
    )
    dismissals = [frequent] * 5 + [rare]
    prompt = build_optimized_prompt(dismissals)

    assert "FREQUENT_PATTERN" in prompt
    assert len(prompt) <= prompt_length_cap()


def test_normalize_message_collapses_whitespace_and_lowercases() -> None:
    assert normalize_message("  Foo   BAR  ") == "foo bar"


def test_default_prompt_length_and_cap() -> None:
    assert default_prompt_length() == len(DEFAULT_SYSTEM_PROMPT)
    assert prompt_length_cap() == 2 * default_prompt_length()
