# Task: SP-038 — Seeded-bug fixtures for model comparison

**Created:** 2026-10-10
**Size:** S

## Review Level: 1

**Assessment:** Test data plus one structural test; no production code. Fixture code is intentionally buggy and stored as `.txt` so it never builds, runs, or gets reviewed in this repo.
**Score:** 1/8 — Blast radius: 0, Pattern novelty: 1, Security: 0, Reversibility: 0

## Mission

Author 14 small, self-contained code-change cases with known ground truth so SP-039 can measure, per model, which seeded bugs stet catches (recall) and how many unexpected findings it raises (noise). SP-037 builds the harness in parallel against the schema below.

### Fixture schema (shared with SP-037 — do not change)

```
scripts/model-eval-cases/<case-id>/
  before/<relative/path>.txt   # committed first; harness strips the trailing .txt
  after/<relative/path>.txt    # full post-change files; a file only in after/ is an added file
  expected.json
```

`expected.json`:

```json
{
  "id": "go-off-by-one",
  "language": "go",
  "clean": false,
  "bugs": [
    {"file": "internal/page/page.go", "lines": [14, 16], "category": "correctness", "description": "slice end uses len+1"}
  ]
}
```

- `id` equals the directory name; `language` ∈ `go`, `typescript`, `python`; `file` is the path **without** `.txt`; `lines` is inclusive 1-based `[start, end]` in the `after/` version, covering only the buggy statement(s).
- `category` is one of stet's `findings.Category` values (`cli/internal/findings/finding.go`); this set uses `correctness`, `security`, and `performance`. Recall is matched on location, not category, so the category only groups results.
- `clean: true` cases have `"bugs": []`.

### Cases (exactly these 14 ids)

| id | language | seeded bug | category |
|----|----------|------------|----------|
| `go-off-by-one` | go | slice/loop bound uses `<=`/`len+1`, reads past end | correctness |
| `go-nil-after-err` | go | `v, _ := f()` (error ignored) then `v.Field` dereference | correctness |
| `go-swallowed-error` | go | `if err != nil { return nil }` drops the error from a write | correctness |
| `go-sql-concat` | go | SQL built with `"... WHERE name = '" + name + "'"` passed to `db.Query` | security |
| `go-map-race` | go | goroutines in a loop write a shared `map` without a mutex | correctness |
| `go-resource-leak` | go | `os.Open` / `http.Get` body never closed on one return path | performance |
| `ts-missing-await` | typescript | `saveUser(u)` returns a Promise, not awaited before `return { ok: true }` | correctness |
| `ts-inverted-guard` | typescript | permission check flipped (`if (user.isAdmin) throw ...`) | security |
| `ts-path-traversal` | typescript | `path.join(baseDir, req.query.file)` read without normalization/containment check | security |
| `py-mutable-default` | python | `def add(item, items=[])` mutates the shared default | correctness |
| `py-bare-except` | python | `except: pass` around a payment/persistence call | correctness |
| `py-int-division` | python | average/percentage computed with `//` truncating a value that must be fractional | correctness |
| `go-clean-refactor` | go | none — extract helper, rename locals, behaviour identical | — |
| `ts-clean-rename` | typescript | none — rename a function and its call sites, add a type annotation | — |

### Authoring rules

- Each case has 1–2 files, 30–120 lines each, realistic package/module layout, idiomatic code that would compile/run apart from the seeded bug. Synthetic only: no real repository code, secrets, or private URLs.
- **Buggy cases:** exactly one seeded bug, introduced by the `before → after` change (the buggy lines are in the diff), **plus one benign control edit** in a separate hunk at least 10 lines away from the bug (e.g. a log message tweak, a new well-written helper). The control edit gives the scorer a place where findings are clearly unexpected.
- No comments that hint at the bug (no `// BUG`, `TODO`, `FIXME`, "unsafe", or similar) in either version.
- Clean cases: the change is non-trivial (≥ 10 changed lines) and correct.
- Keep a short per-case rationale in `expected.json` `description` only; do not write separate docs.

## Dependencies

- **None** (SP-037 lands in parallel; do not edit `scripts/model_eval.py`)

## Context to Read First

**Tier 2 (area context):**

- `spine-tasks/CONTEXT.md`

**Tier 3 (reference docs):**

- `docs/efficacy-tests.md` — H2 (fixture benchmark)
- `cli/internal/findings/finding.go` — allowed `category` values
- `cli/internal/rag/` — languages with symbol resolvers (Go, JS/TS, Python covered here)

## Environment

- **Workspace:** repo root
- **Services required:** none

## File Scope

- `scripts/model-eval-cases/**` (new)
- `scripts/tests/test_model_eval_cases.py` (new)

## Contract

| Field | Value |
|-------|-------|
| testCommand | `python3 -m pytest scripts/tests/test_model_eval_cases.py -q` |
| fileScopeMustChange | `scripts/model-eval-cases/go-off-by-one/expected.json`, `scripts/model-eval-cases/ts-clean-rename/expected.json`, `scripts/tests/test_model_eval_cases.py` |
| fileScopeMustNotChange | `scripts/model_eval.py`, `scripts/tests/test_model_eval.py`, `cli/**`, `extension/**`, `.review/**` |
| completionCriteria | All 14 case ids exist and pass the structural test; every buggy case's bug lines are inside the diff and the control edit is a separate hunk ≥ 10 lines away |

No stet contract gate: every file in this task is `.txt`, `.json`, or a test of fixture structure; `.review/config.toml` excludes `*.txt`/`*.json`, so a stet review would be near-empty.

## Steps

### Step 1: Go cases

- [ ] 6 buggy Go cases + `go-clean-refactor`

### Step 2: TypeScript and Python cases

- [ ] 3 buggy TS cases + `ts-clean-rename`
- [ ] 3 buggy Python cases

### Step 3: Testing & Verification

> ZERO test failures allowed.

`scripts/tests/test_model_eval_cases.py` (stdlib + pytest; must not import `scripts/model_eval.py`) asserts, for the committed tree:

- [ ] Exactly the 14 ids above; `expected.json` `id` == directory name; required keys and types; `language` and `category` in allowed sets
- [ ] Every path under `before/` and `after/` ends in `.txt`; every bug `file` exists as `after/<file>.txt`; `1 <= start <= end <= line count`
- [ ] `clean: true` ⇔ `bugs == []`; 12 buggy, 2 clean
- [ ] Buggy cases: each bug range intersects the changed lines of `after/` (use `difflib.SequenceMatcher` on before/after lines; added files count as all-changed) and at least one changed line lies ≥ 10 lines outside every bug range (control edit)
- [ ] No hint markers (`BUG`, `TODO`, `FIXME`, `XXX`, `HACK`, `unsafe` in comments) in any fixture file
- [ ] Each fixture file is 30–120 lines
- [ ] Run Contract `testCommand`

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing

## Git Commit Convention

- **Fixtures:** `test(SP-038): description`

## Do NOT

- Store fixture code with real extensions (`.go`, `.ts`, `.py`) — it would be built, collected, or reviewed
- Add hints about the bug in code or comments
- Edit `scripts/model_eval.py` or its tests (SP-037)
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
