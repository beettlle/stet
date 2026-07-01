"""Rule-based optimizer: build lessons-learned prompt from dismissal history.

Cross-reference: ``cli/internal/prompt/prompt.go`` ``DefaultSystemPrompt``.
Contract: ``specs/001-implement-sdre-docs/contracts/optimizer-sidecar.md``.
"""

from __future__ import annotations

from collections import defaultdict
from dataclasses import dataclass
import re

from scripts.optimizer.history_loader import DismissalRecord, valid_reason

# Mirror of cli/internal/prompt/prompt.go DefaultSystemPrompt — keep in sync.
DEFAULT_SYSTEM_PROMPT = """You are a Senior Defect Analyst. Review the provided code diff hunk using step-by-step verification. Your goal is to find bugs, security vulnerabilities, and performance issues. Do not comment on style unless it introduces a defect.

The user content is a unified diff hunk: lines starting with "-" are removed, lines starting with "+" are added. Review the change and the resulting code (the + side) for defects. Do not report issues that exist only in the removed lines (-) and are already fixed by the added lines (+). Report only issues in the resulting code or introduced by the change.

## User Intent
(Not provided.)

## Review steps (follow in order)
1. Logic: Check for logic errors (off-by-one, null/zero checks, control flow). Verify variables and functions exist before flagging: if a variable, function, or type is used in the hunk but its definition is not present in the hunk, assume it is valid. Do not report "undefined", "not declared", or "variable not found" for identifiers whose definition is outside the hunk.
2. Security: Check for injection risks, sensitive data exposure, unsafe use of inputs. Before reporting a security or robustness finding, trace back through the code and check for validation in the same function or block. If validation exists, do not report: (a) Path handling: before flagging path traversal or unsafe path construction (e.g. filepath.Join with user input), look for filepath.Rel, filepath.Clean, or path-under-root checks. (b) File operations: before flagging unbounded or unsafe file reads, look for size checks (e.g. Stat, size limits). (c) Indexing/slicing: before flagging potential panics from slice or array access, look for bounds or offset checks.
3. Performance: Check for expensive operations in loops, unnecessary allocations, blocking calls.
4. Output: Emit only high-confidence, actionable findings. Before outputting: if a finding is a nitpick or style-only and not a defect, discard it. Prefer fewer, high-confidence findings over volume.

Report only actionable issues: the developer should be able to apply the suggestion or fix the issue without reverting correct behavior. Do not suggest reverting intentional changes, adding code that already exists, or changing behavior that matches documented design.

## Negative examples (do not report).
- Variable naming: e.g. "Consider renaming x to userData for readability". Required: Do not report; style-only, code is correct.
- Micro-optimization: e.g. "Use a list comprehension here; it is more idiomatic and slightly faster". Required: Do not report; theoretical optimization, not a defect.
- Architectural nit: e.g. "Extract this check into a separate validation module". Required: Do not report; refactor preference, not a correctness issue.
For any of the above, return no finding (or an empty array).

Respond with a single JSON array of findings. Each finding is an object with:
- file (string, required): path to the file
- line (integer, optional if range is set): line number
- range (object, optional): { "start": n, "end": n } for a line span
- severity (string, required): one of "error" | "warning" | "info" | "nitpick"
- category (string, required): one of "bug" | "security" | "correctness" | "performance" | "style" | "maintainability" | "best_practice" | "testing" | "documentation" | "design" | "accessibility"
- confidence (number, required): your certainty 0.0–1.0 (1.0 = definite defect, lower = possible issue)
- message (string, required): review comment
- suggestion (string, optional): suggested fix
- cursor_uri (string, optional): file URI for deep linking
- evidence_lines (array of integers, optional): line numbers in the diff that support this finding.

Set confidence to how sure you are for each finding. Return only the JSON array, no other text. Example: [{"file":"pkg.go","line":10,"severity":"warning","category":"style","confidence":0.9,"message":"Use consistent naming"}]"""

LESSONS_HEADER = "## Lessons learned"

_NEGATIVE_REASONS = frozenset({"false_positive", "wrong_suggestion"})
_VERIFY_REASONS = frozenset({"already_correct"})
_SCOPE_REASONS = frozenset({"out_of_scope"})

_REASON_SECTIONS: tuple[tuple[frozenset[str], str, str], ...] = (
    (
        _NEGATIVE_REASONS,
        "### Negative examples (dismissed findings)",
        "Do not report issues similar to these dismissed findings.",
    ),
    (
        _VERIFY_REASONS,
        "### Verify before reporting",
        "These dismissals indicate the code was already correct. Verify carefully before reporting similar issues.",
    ),
    (
        _SCOPE_REASONS,
        "### Scope filters",
        "These dismissals were out of scope. Do not report similar issues for out-of-scope files or contexts.",
    ),
)

_WHITESPACE_RE = re.compile(r"\s+")


@dataclass(frozen=True)
class AggregatedPattern:
    """Dismissal patterns grouped by reason, category, and normalized message."""

    reason: str
    category: str
    normalized_message: str
    message: str
    count: int
    prompt_contexts: tuple[str, ...]


def default_prompt_length() -> int:
    """Return the character length of the embedded default system prompt."""
    return len(DEFAULT_SYSTEM_PROMPT)


def prompt_length_cap() -> int:
    """Maximum optimized prompt length (2× default)."""
    return 2 * default_prompt_length()


def normalize_message(message: str) -> str:
    """Normalize a finding message for aggregation (lowercase, collapsed whitespace)."""
    collapsed = _WHITESPACE_RE.sub(" ", message.strip())
    return collapsed.lower()


def _aggregate_patterns(dismissals: list[DismissalRecord]) -> list[AggregatedPattern]:
    counts: dict[tuple[str, str, str], int] = defaultdict(int)
    messages: dict[tuple[str, str, str], str] = {}
    contexts: dict[tuple[str, str, str], list[str]] = defaultdict(list)
    seen_context: dict[tuple[str, str, str], set[str]] = defaultdict(set)

    for dismissal in dismissals:
        if not valid_reason(dismissal.reason):
            continue
        normalized = normalize_message(dismissal.message)
        key = (dismissal.reason, dismissal.category, normalized)
        counts[key] += 1
        messages.setdefault(key, dismissal.message.strip())
        context = dismissal.prompt_context.strip()
        if context and context not in seen_context[key]:
            seen_context[key].add(context)
            contexts[key].append(context)

    patterns = [
        AggregatedPattern(
            reason=reason,
            category=category,
            normalized_message=normalized,
            message=messages[(reason, category, normalized)],
            count=count,
            prompt_contexts=tuple(contexts[(reason, category, normalized)]),
        )
        for (reason, category, normalized), count in counts.items()
    ]
    return patterns


def _pattern_sort_key(pattern: AggregatedPattern) -> tuple[int, str, str, str]:
    return (-pattern.count, pattern.reason, pattern.category, pattern.normalized_message)


def _drop_sort_key(pattern: AggregatedPattern) -> tuple[int, str, str, str]:
    return (pattern.count, pattern.reason, pattern.category, pattern.normalized_message)


def _render_pattern_line(pattern: AggregatedPattern, *, include_context: bool) -> str:
    count_suffix = "time" if pattern.count == 1 else "times"
    line = f"- [{pattern.category}] {pattern.message} (dismissed {pattern.count} {count_suffix})"
    if not include_context or not pattern.prompt_contexts:
        return line

    blocks: list[str] = [line]
    for context in pattern.prompt_contexts:
        blocks.append("  Context (dismissed):\n```\n" + context + "\n```")
    return "\n".join(blocks)


def _render_lessons_section(patterns: list[AggregatedPattern]) -> str:
    if not patterns:
        return ""

    by_reason: dict[str, list[AggregatedPattern]] = defaultdict(list)
    for pattern in patterns:
        by_reason[pattern.reason].append(pattern)

    sections: list[str] = [LESSONS_HEADER, ""]
    sections.append(
        "The following patterns were dismissed during prior reviews. Apply this guidance when reviewing similar code.",
    )

    for reason_group, heading, intro in _REASON_SECTIONS:
        group_patterns: list[AggregatedPattern] = []
        for reason in sorted(reason_group):
            group_patterns.extend(by_reason.get(reason, []))
        if not group_patterns:
            continue

        group_patterns.sort(key=_pattern_sort_key)
        sections.extend(["", heading, "", intro, ""])
        include_context = bool(reason_group & _NEGATIVE_REASONS)
        for pattern in group_patterns:
            sections.append(_render_pattern_line(pattern, include_context=include_context))

    return "\n".join(sections).rstrip()


def _total_prompt_length(lessons: str) -> int:
    if not lessons:
        return len(DEFAULT_SYSTEM_PROMPT)
    return len(DEFAULT_SYSTEM_PROMPT) + 2 + len(lessons)


def _truncate_patterns_to_cap(patterns: list[AggregatedPattern]) -> list[AggregatedPattern]:
    remaining = list(patterns)
    while remaining and _total_prompt_length(_render_lessons_section(remaining)) > prompt_length_cap():
        drop_order = sorted(remaining, key=_drop_sort_key)
        remaining.remove(drop_order[0])
    return remaining


def build_optimized_prompt(dismissals: list[DismissalRecord]) -> str:
    """Build optimized system prompt from dismissal records."""
    patterns = _aggregate_patterns(dismissals)
    if not patterns:
        return DEFAULT_SYSTEM_PROMPT

    patterns = _truncate_patterns_to_cap(patterns)
    lessons = _render_lessons_section(patterns)
    if not lessons:
        return DEFAULT_SYSTEM_PROMPT
    return DEFAULT_SYSTEM_PROMPT + "\n\n" + lessons
