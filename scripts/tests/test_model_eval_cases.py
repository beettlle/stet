"""Structural checks for the seeded-bug fixtures in scripts/model-eval-cases.

The fixtures give the model-comparison harness ground truth: which seeded
stet should catch (recall) and where findings are clearly unexpected (noise).
This module asserts only the committed tree shape and diff geometry; it never
imports scripts/model_eval.py.
"""

from __future__ import annotations

import difflib
import json
from collections import defaultdict
from pathlib import Path

import pytest

CASES_DIR = Path(__file__).resolve().parents[1] / "model-eval-cases"

LANGUAGES = {"go", "typescript", "python"}
CATEGORIES = {"correctness", "security", "performance"}

REQUIRED_EXPECTED_KEYS = {"id", "language", "clean", "bugs"}
REQUIRED_BUG_KEYS = {"file", "lines", "category", "description"}

# id -> (language, clean), mirroring the case table in the task packet.
EXPECTED_IDS = {
    "go-off-by-one": ("go", False),
    "go-nil-after-err": ("go", False),
    "go-swallowed-error": ("go", False),
    "go-sql-concat": ("go", False),
    "go-map-race": ("go", False),
    "go-resource-leak": ("go", False),
    "ts-missing-await": ("typescript", False),
    "ts-inverted-guard": ("typescript", False),
    "ts-path-traversal": ("typescript", False),
    "py-mutable-default": ("python", False),
    "py-bare-except": ("python", False),
    "py-int-division": ("python", False),
    "go-clean-refactor": ("go", True),
    "ts-clean-rename": ("typescript", True),
}

HINT_MARKERS = ("bug", "todo", "fixme", "xxx", "hack", "unsafe")

MIN_FILE_LINES = 30
MAX_FILE_LINES = 120
CONTROL_EDIT_MIN_DISTANCE = 10
CLEAN_MIN_CHANGED_LINES = 10


def case_dirs() -> list[Path]:
    if not CASES_DIR.is_dir():
        raise AssertionError(f"missing fixture directory: {CASES_DIR}")
    return sorted(p for p in CASES_DIR.iterdir() if p.is_dir())


def load_expected(case_dir: Path) -> dict:
    return json.loads((case_dir / "expected.json").read_text())


def read_fixtures(case_dir: Path) -> tuple[dict[str, list[str]], dict[str, list[str]]]:
    """Return ({rel path (no .txt): lines}) for before/ and after/."""
    sides: dict[str, dict[str, list[str]]] = {}
    for side in ("before", "after"):
        files: dict[str, list[str]] = {}
        base = case_dir / side
        for path in sorted(base.rglob("*.txt")):
            rel = path.relative_to(base).with_suffix("")
            files[rel.as_posix()] = path.read_text().splitlines()
        sides[side] = files
    return sides["before"], sides["after"]


def diff_opcodes(before_lines: list[str], after_lines: list[str]):
    matcher = difflib.SequenceMatcher(a=before_lines, b=after_lines, autojunk=False)
    return matcher.get_opcodes()


def changed_after_lines(before_lines: list[str], after_lines: list[str]) -> set[int]:
    """1-based after/ line numbers touched by the before -> after change."""
    return {
        j + 1
        for tag, _i1, _i2, j1, j2 in diff_opcodes(before_lines, after_lines)
        if tag in ("replace", "insert")
        for j in range(j1, j2)
    }


def distance_to_ranges(line: int, ranges: list[tuple[int, int]]) -> int:
    """Lines from line to the nearest inclusive range; 0 when inside one."""
    distances = []
    for start, end in ranges:
        if start <= line <= end:
            return 0
        distances.append(start - line if line < start else line - end)
    return min(distances)


def test_exactly_the_expected_case_ids():
    assert {p.name for p in case_dirs()} == set(EXPECTED_IDS)


def test_buggy_clean_split():
    expected = {case.name: load_expected(case) for case in case_dirs()}
    clean = [entry["clean"] for entry in expected.values()]
    assert clean.count(True) == 2
    assert clean.count(False) == 12
    for entry in expected.values():
        assert entry["clean"] == (entry["bugs"] == [])


@pytest.mark.parametrize("case_dir", case_dirs(), ids=lambda p: p.name)
def test_expected_json_schema_and_values(case_dir):
    expected = load_expected(case_dir)
    assert REQUIRED_EXPECTED_KEYS <= set(expected)
    assert expected["id"] == case_dir.name
    assert expected["id"] in EXPECTED_IDS
    language, clean = EXPECTED_IDS[case_dir.name]
    assert expected["language"] == language
    assert expected["language"] in LANGUAGES
    assert expected["clean"] is clean
    assert isinstance(expected["bugs"], list)
    for bug in expected["bugs"]:
        assert REQUIRED_BUG_KEYS <= set(bug)
        assert bug["category"] in CATEGORIES
        assert isinstance(bug["file"], str) and bug["file"]
        assert isinstance(bug["lines"], list) and len(bug["lines"]) == 2
        assert all(isinstance(n, int) and not isinstance(n, bool) for n in bug["lines"])
        assert isinstance(bug["description"], str) and bug["description"]


@pytest.mark.parametrize("case_dir", case_dirs(), ids=lambda p: p.name)
def test_fixture_files_end_in_txt(case_dir):
    for side in ("before", "after"):
        base = case_dir / side
        assert base.is_dir(), f"missing {side}/ directory"
        for path in base.rglob("*"):
            if path.is_file():
                assert path.suffix == ".txt", f"non-txt fixture file: {path}"


@pytest.mark.parametrize("case_dir", case_dirs(), ids=lambda p: p.name)
def test_bug_files_exist_and_line_ranges_in_bounds(case_dir):
    expected = load_expected(case_dir)
    _, after_files = read_fixtures(case_dir)
    for bug in expected["bugs"]:
        assert bug["file"] in after_files, f"missing after/{bug['file']}.txt"
        start, end = bug["lines"]
        line_count = len(after_files[bug["file"]])
        assert 1 <= start <= end <= line_count, (
            f"bug lines {bug['lines']} out of bounds for {bug['file']} "
            f"({line_count} lines)"
        )


@pytest.mark.parametrize("case_dir", case_dirs(), ids=lambda p: p.name)
def test_bug_lines_in_diff_and_control_edit_far_away(case_dir):
    expected = load_expected(case_dir)
    if expected["clean"]:
        pytest.skip("clean case has no seeded bug")
    assert len(expected["bugs"]) == 1, "exactly one seeded bug per buggy case"
    before_files, after_files = read_fixtures(case_dir)

    ranges_by_file: dict[str, list[tuple[int, int]]] = defaultdict(list)
    for bug in expected["bugs"]:
        ranges_by_file[bug["file"]].append((bug["lines"][0], bug["lines"][1]))

    far_changed: list[str] = []
    for rel, after_lines in after_files.items():
        before_lines = before_files.get(rel)
        if before_lines is None:
            changed = set(range(1, len(after_lines) + 1))  # added file: all changed
        else:
            changed = changed_after_lines(before_lines, after_lines)
        ranges = ranges_by_file.get(rel, [])
        for line in sorted(changed):
            if not ranges or (
                distance_to_ranges(line, ranges) >= CONTROL_EDIT_MIN_DISTANCE
            ):
                far_changed.append(f"{rel}:{line}")

    assert far_changed, (
        "no changed line lies >= "
        f"{CONTROL_EDIT_MIN_DISTANCE} lines outside every bug range; "
        "the benign control edit is missing or too close to the bug"
    )


@pytest.mark.parametrize("case_dir", case_dirs(), ids=lambda p: p.name)
def test_clean_change_is_nontrivial(case_dir):
    expected = load_expected(case_dir)
    if not expected["clean"]:
        pytest.skip("buggy case")
    before_files, after_files = read_fixtures(case_dir)
    changed = 0
    for rel, after_lines in after_files.items():
        before_lines = before_files.get(rel, [])
        for tag, i1, i2, j1, j2 in diff_opcodes(before_lines, after_lines):
            if tag != "equal":
                changed += (i2 - i1) + (j2 - j1)
    assert changed >= CLEAN_MIN_CHANGED_LINES, (
        f"clean case change touches only {changed} lines, "
        f"want >= {CLEAN_MIN_CHANGED_LINES}"
    )


@pytest.mark.parametrize("case_dir", case_dirs(), ids=lambda p: p.name)
def test_no_hint_markers_in_fixtures(case_dir):
    for side in ("before", "after"):
        for path in sorted((case_dir / side).rglob("*.txt")):
            for lineno, line in enumerate(path.read_text().splitlines(), start=1):
                lowered = line.lower()
                for marker in HINT_MARKERS:
                    assert marker not in lowered, (
                        f"{path.relative_to(case_dir)}:{lineno} "
                        f"contains hint marker {marker!r}"
                    )


@pytest.mark.parametrize("case_dir", case_dirs(), ids=lambda p: p.name)
def test_fixture_files_have_realistic_length(case_dir):
    for side in ("before", "after"):
        for path in sorted((case_dir / side).rglob("*.txt")):
            count = len(path.read_text().splitlines())
            assert MIN_FILE_LINES <= count <= MAX_FILE_LINES, (
                f"{path.relative_to(case_dir)} has {count} lines, "
                f"want {MIN_FILE_LINES}..{MAX_FILE_LINES}"
            )
