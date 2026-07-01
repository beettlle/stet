"""Unit tests for scripts/optimizer/prompt_validator.py."""

from __future__ import annotations

from pathlib import Path

import pytest

from scripts.optimizer.prompt_validator import (
    PromptValidationError,
    assert_valid_prompt,
    validate_prompt,
)

FIXTURES_DIR = Path(__file__).resolve().parent / "fixtures"
DEFAULT_PROMPT_PATH = FIXTURES_DIR / "default_prompt.txt"


@pytest.fixture
def default_prompt() -> str:
    return DEFAULT_PROMPT_PATH.read_text(encoding="utf-8")


def test_default_prompt_fixture_passes(default_prompt: str) -> None:
    result = validate_prompt(default_prompt)
    assert result.ok, result.violations
    assert_valid_prompt(default_prompt)


def test_empty_prompt_rejected() -> None:
    result = validate_prompt("")
    assert not result.ok
    assert "prompt is empty" in result.violations


def test_whitespace_only_prompt_rejected() -> None:
    result = validate_prompt("   \n\t  ")
    assert not result.ok
    assert "prompt is empty" in result.violations


def test_missing_json_array_instruction_rejected(default_prompt: str) -> None:
    broken = default_prompt.replace("Respond with a single JSON array of findings.", "")
    result = validate_prompt(broken)
    assert not result.ok
    assert "missing JSON array output instruction" in result.violations


def test_missing_json_only_return_rejected(default_prompt: str) -> None:
    broken = default_prompt.replace("Return only the JSON array, no other text.", "")
    result = validate_prompt(broken)
    assert not result.ok
    assert "missing JSON-only return instruction" in result.violations


def test_missing_severity_enum_rejected(default_prompt: str) -> None:
    broken = default_prompt.replace(
        '"error" | "warning" | "info" | "nitpick"',
        '"error" | "warning"',
    )
    result = validate_prompt(broken)
    assert not result.ok
    assert "missing severity enum" in result.violations


def test_missing_category_enum_rejected(default_prompt: str) -> None:
    broken = default_prompt.replace(
        '"bug" | "security" | "correctness" | "performance" | "style"',
        '"bug" | "security"',
    )
    result = validate_prompt(broken)
    assert not result.ok
    assert "missing category enum" in result.violations


def test_missing_diff_removed_rule_rejected(default_prompt: str) -> None:
    broken = default_prompt.replace(
        'lines starting with "-" are removed, lines starting with "+" are added.',
        'lines starting with "+" are added.',
    )
    result = validate_prompt(broken)
    assert not result.ok
    assert "missing diff removed-lines rule (-)" in result.violations


def test_missing_diff_added_rule_rejected(default_prompt: str) -> None:
    broken = default_prompt.replace(
        'lines starting with "-" are removed, lines starting with "+" are added.',
        'lines starting with "-" are removed.',
    )
    result = validate_prompt(broken)
    assert not result.ok
    assert "missing diff added-lines rule (+)" in result.violations


def test_missing_diff_plus_side_rule_rejected(default_prompt: str) -> None:
    broken = default_prompt.replace(
        "Review the change and the resulting code (the + side) for defects.",
        "Review the change for defects.",
    )
    result = validate_prompt(broken)
    assert not result.ok
    assert "missing diff + side review rule" in result.violations


def test_missing_removed_only_rule_rejected(default_prompt: str) -> None:
    broken = default_prompt.replace(
        "Do not report issues that exist only in the removed lines (-) and are already fixed by the added lines (+).",
        "Do not report issues that are already fixed by the added lines (+).",
    )
    result = validate_prompt(broken)
    assert not result.ok
    assert "missing rule to ignore removed-only issues" in result.violations


def test_missing_negative_examples_rejected(default_prompt: str) -> None:
    start = default_prompt.index("## Negative examples")
    end = default_prompt.index("Respond with a single JSON array")
    broken = default_prompt[:start] + default_prompt[end:]
    result = validate_prompt(broken)
    assert not result.ok
    assert "missing negative examples section" in result.violations


def test_assert_valid_prompt_raises(default_prompt: str) -> None:
    with pytest.raises(PromptValidationError) as exc_info:
        assert_valid_prompt("")
    assert exc_info.value.violations
