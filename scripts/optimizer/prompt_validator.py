"""Blocking contract check for optimized Stet system prompts.

Enforces preservation of structured findings instructions required by
specs/001-implement-sdre-docs/data-model.md (JSON schema, diff +/- rules,
negative examples). Cross-reference: cli/internal/prompt/prompt.go DefaultSystemPrompt.
"""

from __future__ import annotations

from dataclasses import dataclass
import re


@dataclass(frozen=True)
class ValidationResult:
    """Outcome of prompt contract validation."""

    ok: bool
    violations: tuple[str, ...]


class PromptValidationError(Exception):
    """Raised when a prompt fails contract validation."""

    def __init__(self, violations: tuple[str, ...] | list[str]) -> None:
        self.violations = tuple(violations)
        message = (
            "; ".join(self.violations) if self.violations else "prompt validation failed"
        )
        super().__init__(message)


# Each tuple is (violation message, pattern). All patterns must match.
_CONTRACT_CHECKS: tuple[tuple[str, re.Pattern[str]], ...] = (
    (
        "missing JSON array output instruction",
        re.compile(r"single\s+JSON\s+array", re.IGNORECASE),
    ),
    (
        "missing JSON-only return instruction",
        re.compile(r"Return\s+only\s+the\s+JSON\s+array", re.IGNORECASE),
    ),
    (
        "missing severity enum",
        re.compile(
            r'"error"\s*\|\s*"warning"\s*\|\s*"info"\s*\|\s*"nitpick"',
        ),
    ),
    (
        "missing category enum",
        re.compile(
            r'"bug"\s*\|\s*"security"\s*\|\s*"correctness"\s*\|\s*"performance"',
        ),
    ),
    (
        "missing diff removed-lines rule (-)",
        re.compile(
            r'lines?\s+starting\s+with\s+"-"\s+are\s+removed',
            re.IGNORECASE,
        ),
    ),
    (
        "missing diff added-lines rule (+)",
        re.compile(
            r'lines?\s+starting\s+with\s+"\+"\s+are\s+added',
            re.IGNORECASE,
        ),
    ),
    (
        "missing diff + side review rule",
        re.compile(r"resulting\s+code\s*\(\s*the\s*\+\s*side\s*\)", re.IGNORECASE),
    ),
    (
        "missing rule to ignore removed-only issues",
        re.compile(r"removed\s+lines?\s*\(-\)", re.IGNORECASE),
    ),
    (
        "missing negative examples section",
        re.compile(r"##\s*Negative\s+examples", re.IGNORECASE),
    ),
)


def validate_prompt(prompt: str) -> ValidationResult:
    """Validate prompt text against the Stet JSON finding contract."""
    if not prompt or not prompt.strip():
        return ValidationResult(False, ("prompt is empty",))

    violations: list[str] = []
    for message, pattern in _CONTRACT_CHECKS:
        if not pattern.search(prompt):
            violations.append(message)

    return ValidationResult(ok=not violations, violations=tuple(violations))


def assert_valid_prompt(prompt: str) -> None:
    """Raise PromptValidationError if prompt violates the contract."""
    result = validate_prompt(prompt)
    if not result.ok:
        raise PromptValidationError(result.violations)
