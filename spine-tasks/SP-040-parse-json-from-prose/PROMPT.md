# Task: SP-040 — Recover findings JSON from prose responses

**Created:** 2026-10-10
**Size:** S

## Review Level: 1

**Assessment:** Parser fallback plus prompt wording; strict-JSON path unchanged, retry still one extra request.
**Score:** 2/8 — Blast radius: 1, Pattern novelty: 1, Security: 0, Reversibility: 0

## Mission

`stet run` fails with `review: parse failed (after retry): parse findings: invalid character 'L' looking for beginning of value` when the model (observed: `qwen/qwen3-coder-next` via LM Studio) writes prose analysis and only then a fenced ```` ```json [...] ``` ```` block. `stripMarkdownCodeFence` (`cli/internal/review/parse.go`) only strips a fence at the very start of the response, and the retry in `cli/internal/review/review.go` (~line 326) resends the identical prompt, so the model writes prose again. Observed shape (truncated at 4095 tokens, continuation ends with the fenced array):

````text
Looking at the code, I need to check ...
... several paragraphs, may contain `inline code`, [brackets], and {braces} ...
```json
[{"file":"x.py","line":12,"severity":"warning","category":"bug","message":"...","confidence":0.8}]
```
````

The model also treats the "Do not report issues similar to" suppression examples and the dismissed-shadow block as hints and discusses them in prose.

**Behaviour:**

- **Extraction fallback (`cli/internal/review/extract_json.go`, new):** when the existing strict parse in `ParseFindingsResponse` fails, try to recover a findings payload from the text before returning the error:
  1. Fenced blocks (```` ``` ```` with optional language tag), newest (last) first.
  2. Then balanced JSON spans starting at `[` or `{`, scanning from the end of the text backwards over start positions; decode each candidate with `json.Decoder` (so trailing text is ignored) and accept only an array of finding objects or a `{"findings": [...]}` wrapper.
  3. The first candidate that decodes into findings wins and goes through the same normalize/validate path as today. An empty array `[]` is a valid result.
  4. If nothing decodes, return the original strict-parse error (unchanged message) so callers and tests keep their error text.
  Bound the work: cap candidates tried (e.g. 64) and do not scan responses larger than the existing response size (no new limits on the strict path).
- **Strict path unchanged:** responses that parse today must produce identical findings; `MergeFindingsFromParts` keeps its behaviour and benefits automatically because it calls `ParseFindingsResponse`.
- **Retry hardening (`review.go`):** the single retry after a parse failure appends to the user message an instruction that the previous reply was not valid JSON and the reply must be only the JSON array, no prose and no code fences. Still exactly one extra `Generate` call. The trace (`--trace`) records that the retry used the JSON-only instruction.
- **Prompt wording (`cli/internal/prompt/prompt.go`):**
  - Suppression header: replace `## Do not report issues similar to` with a header that states these are previously dismissed false positives, not hints, and must not be discussed (for example `## Previously dismissed false positives (do not report or discuss)`). Keep it a single constant used by both `AppendSuppressionExamples` and `EstimateSuppressionBlock`.
  - Shadow intro in `AppendPromptShadows`: same framing (dismissed, not hints, do not discuss).
  - End of the system prompt output-format section: one closing sentence, "Respond with only the JSON array. Do not include analysis, prose, or Markdown." Do not otherwise rewrite the default prompt.
  - Update the header assertions in `prompt_test.go`, `review_test.go`, and `run_test.go` to the new constant text (reference the constant where the package allows it).

**Stet-gated:** the contract ends with `scripts/spine-stet-contract-run.sh default`. If it fails with active findings: run `stet list`, fix real defects, and dismiss the rest with `stet dismiss <id> <reason>` (`false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`). Record any stet reviewer bug under Discoveries in STATUS.md. Never dismiss a real defect without fixing it. Note: the contract runs the stet binary installed on PATH, which does not yet contain this fix; if the contract review itself fails with the parse error this task fixes, record it in STATUS.md Blockers and stop (the operator handles it).

## Dependencies

- **None**

## Context to Read First

**Tier 2 (area context):**

- `spine-tasks/CONTEXT.md` — "Stet in spine (dogfooding)" section

**Tier 3 (reference docs):**

- `cli/internal/review/parse.go` — `ParseFindingsResponse`, `MergeFindingsFromParts`, `stripMarkdownCodeFence`, `normalizeAndFilterFindings`
- `cli/internal/review/review.go` — parse / continuation / retry block (~lines 285–345)
- `cli/internal/prompt/prompt.go` — default system prompt output format (~line 58), `AppendPromptShadows` (~line 414), `suppressionExamplesHeader`, `AppendSuppressionExamples`, `EstimateSuppressionBlock`
- `cli/internal/review/review_test.go` — `TestReviewHunk_retryThenSuccess` (expects exactly 2 generate calls)

## Environment

- **Workspace:** `cli/` Go module
- **Services required:** LM Studio at `http://127.0.0.1:1234` for the stet contract step only (unit tests use fakes)

## File Scope

- `cli/internal/review/parse.go`
- `cli/internal/review/parse_test.go`
- `cli/internal/review/extract_json.go`
- `cli/internal/review/extract_json_test.go`
- `cli/internal/review/review.go`
- `cli/internal/review/review_test.go`
- `cli/internal/prompt/prompt.go`
- `cli/internal/prompt/prompt_test.go`
- `cli/internal/run/run_test.go`

## Contract

| Field | Value |
|-------|-------|
| testCommand | `cd cli && go test ./internal/review/... ./internal/prompt/... ./internal/run/... -count=1 && go vet ./... && cd .. && bash scripts/spine-stet-contract-run.sh default` |
| fileScopeMustChange | `cli/internal/review/parse.go`, `cli/internal/review/extract_json.go`, `cli/internal/review/extract_json_test.go`, `cli/internal/review/review.go`, `cli/internal/prompt/prompt.go` |
| fileScopeMustNotChange | `extension/**`, `scripts/**`, `.review/**`, `cli/go.mod`, `cli/go.sum` |
| completionCriteria | Prose-then-fenced-JSON and prose-then-bare-JSON responses parse into the embedded findings; newest valid block wins; strict-JSON responses parse identically to before; unrecoverable text still returns the original error; retry sends exactly one extra Generate with the JSON-only instruction; suppression and shadow sections use the new framing; stet contract review exits 0; per-file coverage ≥ 72% for changed Go files |

## Steps

### Step 1: Extraction fallback

- [ ] `extract_json.go`: fenced-block candidates (newest first), then balanced-span candidates via `json.Decoder`, bounded candidate count
- [ ] Wire into `ParseFindingsResponse` after strict parse fails; return original error when nothing decodes

### Step 2: Retry and prompt hardening

- [ ] Retry appends JSON-only instruction to the user message; trace notes it
- [ ] New suppression header constant, shadow intro, closing output-format sentence
- [ ] Update header assertions in `prompt_test.go`, `review_test.go`, `run_test.go`

### Step 3: Testing & Verification

> ZERO test failures allowed.

- [ ] Table tests for the extractor: prose then fenced `json` array (the shape in Mission); prose then bare array; prose containing `[x]` / `{y}` / inline code before the real array; two fenced blocks where the last is valid (last wins); last block truncated/invalid with an earlier valid block (earlier wins); `{"findings":[...]}` wrapper in prose; `[]` in prose; pure prose (original error returned); strict array and leading-fence inputs unchanged
- [ ] `MergeFindingsFromParts` test: first part prose, last part fenced array
- [ ] `review_test.go`: retry user message contains the JSON-only instruction; still exactly 2 Generate calls in `TestReviewHunk_retryThenSuccess`; a first response of prose + fenced JSON succeeds with no retry (1 Generate call)
- [ ] Run Contract `testCommand` (includes the stet review; triage findings as described in Mission)
- [ ] Coverage gate: `cd cli && go test ./... -coverprofile=../coverage.out -count=1 && cd .. && bash scripts/check-coverage.sh coverage.out`

## Completion Criteria

- [ ] All steps complete
- [ ] All tests passing

## Git Commit Convention

- **Implementation:** `fix(SP-040): description`
- **Tests:** `test(SP-040): description`

## Do NOT

- Change the strict-JSON parse results or the error text for unrecoverable responses
- Add more than one retry request, or change continuation-round behaviour
- Rewrite the default system prompt beyond the closing sentence
- Edit `.review/config.toml` or `scripts/**`
- Modify `.spine/`, `AGENTS.md`, `CLAUDE.md`, `.gitnexus/`
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
