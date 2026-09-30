# Task: SP-020 — Research: rules compliance via System One

**Created:** 2026-09-30
**Size:** M
**Status note:** Deferred — not scheduled in any wave until the operator promotes it.

## Review Level: 1

**Assessment:** Research spike; the only committed deliverable is a document, but its recommendation shapes future product work.
**Score:** 2/8 — Blast radius: 0, Pattern novelty: 2, Security: 0, Reversibility: 0

## Mission

Decide, with evidence, whether stet should use a System One model to check that review findings and suggested fixes **do not contradict the project's `.cursor/rules`**. This is research only: produce a written go/no-go recommendation, not a product feature.

**Background (System One / OpenJev):** OpenJev ([razorback16/openjev](https://github.com/razorback16/openjev)) exposes `POST /v1/systemone` with `{state, model, questions}`; question types are `noul` (yes/no → P(yes)), `choice` (named options → probabilities), and `score` (ordered levels). The server reads questions in chunks of about 12 per pass. Warm latency ~100 ms per request. After SP-014–SP-019 and SP-021–SP-025, stet has a System One client (`cli/internal/systemone`), a batch verifier (`review.VerifyFindingsSystemOne`), and an evaluation harness (`stet critic-eval` / `cli/internal/criticeval`).

**Why this is hard:** stet currently injects rules into the review prompt with a ~1,000-token budget (`rules.MaxRuleTokens`) and truncates the rest. Rule files are long (e.g. `.cursor/rules/general-llm-anti-patterns.mdc` is ~1,200 lines of mixed guidance), so a single "does this suggestion violate the rules?" question over whole files is unlikely to work.

**Questions to answer:**

1. **Atomic premises:** Can `.mdc` rules be split into short, single-statement premises (e.g. "Repositories contain no business logic")? How: headings/bullets heuristics, an LLM pre-pass, or manual curation? What fraction of rules survive as checkable statements?
2. **Cost:** Questions scale as findings × applicable premises. Using file-glob filtering (`rules.FilterRules`), what is the typical and worst-case question count per hunk in this repo, and the measured latency?
3. **Accuracy:** On a small hand-labeled set (≥ 20 suggestion/premise pairs, half violating), how well does System One flag violations? Compare `noul` per premise vs one `choice` question listing premises.
4. **Prompt injection:** How easily does text inside the diff (e.g. a comment claiming compliance) flip the answer? Test at least 5 adversarial variants.
5. **Placement:** Should a positive result become a filter (drop findings), an annotation (flag findings), or a pre-prompt rule selector?

## Dependencies

- **Task:** SP-025 (`stet critic-eval` command; transitively the full System One critic chain)
- **External:** a running OpenJev (or other System One) server for measurements

## Context to Read First

**Tier 2 (area context):**

- `spine-tasks/CONTEXT.md`

**Tier 3 (reference docs):**

- `cli/internal/rules/rules.go`, `cli/internal/rules/loader.go` — rule parsing, `FilterRules`, `MaxRuleTokens`
- `cli/internal/prompt/prompt.go` — `AppendCursorRules`
- `cli/internal/review/critic_systemone.go`, `cli/internal/criticeval/criticeval.go`
- `docs/review-process-internals.md` §7.3

## Environment

- **Workspace:** repo root
- **Services required:** System One server (default `http://127.0.0.1:8080`)

## File Scope

- `docs/research/systemone-rules-compliance.md` (new; user-requested research deliverable)

Throwaway prototype code and labeled pairs may be created locally for measurement but are **not committed**; the document must include enough detail (commands, request shapes, dataset description) to reproduce the results.

## Contract

| Field | Value |
|-------|-------|
| testCommand | `test -s docs/research/systemone-rules-compliance.md && grep -q '## Recommendation' docs/research/systemone-rules-compliance.md` |
| fileScopeMustChange | `docs/research/systemone-rules-compliance.md` |
| fileScopeMustNotChange | `cli/**`, `extension/**`, `scripts/**` |
| completionCriteria | Document answers all five questions with measurements and ends with a go/no-go recommendation; if go, lists proposed follow-up SP tasks with sizes |

## Steps

### Step 0: Design the experiment

- [ ] Choose premise-extraction approach(es) and labeled-set construction; record in STATUS Discoveries

> ⚠️ Hydrate: expand Step 1 once the approach is chosen.

### Step 1: Premise extraction and cost measurement

- [ ] Extract atomic premises from this repo's `.cursor/rules`; report yield
- [ ] Measure question counts and latency per hunk over a sample of recent diffs

### Step 2: Accuracy and injection experiments

- [ ] Run `noul`-per-premise and `choice` variants on the labeled set; report accuracy/AUROC
- [ ] Run ≥ 5 prompt-injection variants; report flip rate

### Step 3: Write the document

- [ ] `docs/research/systemone-rules-compliance.md` with sections: Summary, Method, Results (one per question), Risks, `## Recommendation`, Follow-up tasks (if go)

### Step 4: Testing and verification

> ZERO test failures allowed unless PROMPT documents a known baseline.

- [ ] Run Contract `testCommand`
- [ ] Confirm no files outside File Scope are committed (`git status`)

## Completion Criteria

- [ ] All steps complete
- [ ] All five questions answered with data
- [ ] Recommendation is explicit (go / no-go / go-with-conditions)

## Git Commit Convention

- **Docs:** `docs(SP-020): description`

## Do NOT

- Merge production code or new CLI commands
- Edit `.cursor/rules/**`
- Commit prototype code or datasets containing real user code
- Expand scope beyond this PROMPT—log tech debt in `spine-tasks/CONTEXT.md`

---

## Amendments (Added During Execution)
