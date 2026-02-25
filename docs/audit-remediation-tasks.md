# Audit remediation task list

This document turns the issues in [docs/audit-issues.md](docs/audit-issues.md) into **small, testable tasks** suitable for an LLM. Implement only the tasks listed below; verify each with the given acceptance commands. Do not change code for issues in the "Do not implement" section.

---

## Do not implement

These issue numbers are **false positives** or **intentional**; leave the code as-is.

- **#3** – erruser.go:18 – Error() already has nil check.
- **#10, #15, #16, #19, #23** – Import path "stet/cli/internal/..." is correct (module in go.mod is `stet`); do not change to github.com/...
- **#13, #14** – Package name `goresolver` in directory `go/` is intentional (reserved word).
- **#17** – Package name `python` matches directory `cli/internal/rag/python`; valid.
- **#20** – critic.go:107 KeepAlive handling is correct when opts.KeepAlive is nil.
- **#24** – stats/quality_test.go: package `stats` matches directory `stats`.
- **#33** – extension contract.ts: `accessibility` is already in CATEGORIES (line 26).
- **#74** – critic.go:107 – Same as #20; false positive.
- **#146** – scripts/check-coverage.sh has shebang on line 1.

---

## ERROR tasks (fix first)

### T-ERR-01 – Add missing imports in Go test files

| Field | Value |
| ----- | ----- |
| **Issues** | #4, #6, #9, #11, #32 |
| **Files** | `cli/internal/erruser/erruser_test.go`, `cli/internal/findings/abstention_test.go`, `cli/internal/findings/strictness_test.go`, `cli/internal/hunkid/hunkid_test.go`, `cli/internal/review/parse_test.go` |
| **Description** | Add missing imports: `errors` (erruser_test), `testing` (abstention_test, hunkid_test), `strings` (strictness_test, parse_test). Ensure each test file compiles and uses the imported packages. |
| **Acceptance** | `cd cli && go build ./...` and `go test ./cli/internal/erruser/... ./cli/internal/findings/... ./cli/internal/hunkid/... ./cli/internal/review/...` |

### T-ERR-02 – Trailing newlines / end-of-file

| Field | Value |
| ----- | ----- |
| **Issues** | #5, #27, #28, #60 |
| **Files** | `cli/internal/findings/abstention.go`, `cli/internal/version/version.go`, `cli/internal/findings/finding.go`, `cli/internal/history/schema.go` |
| **Description** | Ensure a newline at end of the function body or end of file in each listed file. Fix any missing newline at EOF. |
| **Acceptance** | `cd cli && go build ./...`; no new linter errors on these files. |

### T-ERR-03 – findings: strictness unreachable default and evidence overlap

| Field | Value |
| ----- | ----- |
| **Issues** | #7, #8 |
| **Files** | `cli/internal/findings/evidence.go`, `cli/internal/findings/strictness.go` |
| **Description** | In strictness.go: remove or fix the unreachable default case in ResolveStrictness (it returns applyFP=true; valid norm values already handled). In evidence.go: clarify or fix the range-overlap condition/comment in FilterByHunkLines for edge cases where ranges touch but do not overlap. |
| **Acceptance** | `go test ./cli/internal/findings/...` |

### T-ERR-04 – findings: validation and parse normalization

| Field | Value |
| ----- | ----- |
| **Issues** | #29, #30, #31 |
| **Files** | `cli/internal/findings/finding_test.go`, `cli/internal/findings/validation.go`, `cli/internal/review/parse.go` |
| **Description** | In finding_test.go: add or use CategoryAccessibility so test case 'valid_accessibility' is defined. In validation.go: document that Normalize sets invalid category to CategoryBug and Validate accepts it (CategoryBug is in validCategories). In parse.go: in the single-object fallback path, call Normalize before validation so invalid categories are normalized. |
| **Acceptance** | `go test ./cli/internal/findings/... ./cli/internal/review/...` |

### T-ERR-05 – findings: validation_test nil and rag_test nil vs empty

| Field | Value |
| ----- | ----- |
| **Issues** | #18, #49 |
| **Files** | `cli/internal/rag/rag_test.go`, `cli/internal/findings/validation_test.go` |
| **Description** | In rag_test.go: align TestResolveSymbols_unknownExtension_returnsNil with implementation—either expect empty slice when no resolver is found or change implementation to return nil. In validation_test.go: guard or adjust TestFindingNormalize_nilNoop so calling Normalize on a nil pointer does not panic (e.g. skip test for nil or test via interface). |
| **Acceptance** | `go test ./cli/internal/rag/... ./cli/internal/findings/...` |

### T-ERR-06 – Config timeout and diff parse_test

| Field | Value |
| ----- | ----- |
| **Issues** | #1, #2 |
| **Files** | `cli/internal/config/config.go`, `cli/internal/diff/parse_test.go` |
| **Description** | In config: search for any comment that says "5 minutes" and fix to match the 15-minute default (constant at line 123). In parse_test.go: fix or verify the diff string literal in TestParseUnifiedDiff_singleFileSingleHunk (ensure closing backtick if missing). |
| **Acceptance** | `go test ./cli/internal/config/... ./cli/internal/diff/...` |

### T-ERR-07 – minify: panic guard and test expected output

| Field | Value |
| ----- | ----- |
| **Issues** | #12, #64 |
| **Files** | `cli/internal/minify/minify.go`, `cli/internal/minify/minify_test.go` |
| **Description** | In minify.go: ensure line access (e.g. line[0:1]) is safe when len(line)==1; add guard or document why empty lines are already handled above. In minify_test.go: fix the expected output for the test case 'header plus lines with leading whitespace' so it matches actual behavior. |
| **Acceptance** | `go test ./cli/internal/minify/...` |

### T-ERR-08 – session lock: Windows nil deref and Unix fd leak

| Field | Value |
| ----- | ----- |
| **Issues** | #22, #81 |
| **Files** | `cli/internal/session/lock_windows.go`, `cli/internal/session/lock_unix.go` |
| **Description** | In lock_windows.go: fix the potential nil pointer dereference in the error handling path (line 45). In lock_unix.go: ensure the file descriptor is closed if syscall.Flock fails (line 25) to avoid resource leak. |
| **Acceptance** | `cd cli && go build ./...`; `go test ./cli/internal/session/...` (if tests exist and run on your platform). |

### T-ERR-09 – trace: Section nil and trace_test logic

| Field | Value |
| ----- | ----- |
| **Issues** | #26, #90 |
| **Files** | `cli/internal/trace/trace.go`, `cli/internal/trace/trace_test.go` |
| **Description** | In trace.go: guard the Section method against nil receiver (line 25). In trace_test.go: fix TestNew_nilWriter_returnsTracer so it does not use t.Fatal in a way that blocks other tests; assert expected behavior (e.g. tracer may or may not be nil) without stopping the test run. |
| **Acceptance** | `go test ./cli/internal/trace/...` |

### T-ERR-10 – tokens: integer overflow and test

| Field | Value |
| ----- | ----- |
| **Issues** | #25, #89 |
| **Files** | `cli/internal/tokens/estimate.go`, `cli/internal/tokens/estimate_test.go` |
| **Description** | In estimate.go: prevent or handle integer overflow when promptTokens + responseReserve is near math.MaxInt (line 47). In estimate_test.go: adjust the 'overflow_returns_warning' test so it does not trigger undefined behavior; use safe inputs or mock. |
| **Acceptance** | `go test ./cli/internal/tokens/...` |

### T-ERR-11 – rules_test rename

| Field | Value |
| ----- | ----- |
| **Issues** | #21 |
| **Files** | `cli/internal/rules/rules_test.go` |
| **Description** | Rename the test function TestParseMDC_globsString_contentParsed to match the pattern of other test functions (e.g. TestParseMDC_globsString). |
| **Acceptance** | `go test ./cli/internal/rules/...` |

### T-ERR-12 – Devcontainer Dockerfile checksum

| Field | Value |
| ----- | ----- |
| **Issues** | #34 |
| **Files** | `.devcontainer/Dockerfile` |
| **Description** | Add checksum verification for any script or artifact installed from external sources. If checksums are not available, document why in a comment. |
| **Acceptance** | Dockerfile builds; or comment documents why checksum is omitted. |

---

## WARNING tasks

### T-WRN-01 – main.go: commitmsg nil and NumCtx

| Field | Value |
| ----- | ----- |
| **Issues** | #37, #38 |
| **Files** | `cli/cmd/stet/main.go` (lines 1027, 1029) |
| **Description** | Guard the commitmsg.Suggest call against nil (line 1027). Ensure NumCtx assignment when cfg.NumCtx is 0 is correct (line 1029)—use 0 or a sensible default as intended. |
| **Acceptance** | `cd cli && go build ./...`; run relevant main commands (e.g. start/run) if covered by tests. |

### T-WRN-02 – benchmark division by zero

| Field | Value |
| ----- | ----- |
| **Issues** | #39 |
| **Files** | `cli/internal/benchmark/benchmark.go` |
| **Description** | Guard EvalRateTPS calculation (line 70) against division by zero. |
| **Acceptance** | `go test ./cli/internal/benchmark/...` |

### T-WRN-03 – commitmsg UTF-8 truncation

| Field | Value |
| ----- | ----- |
| **Issues** | #40 |
| **Files** | `cli/internal/commitmsg/commitmsg.go` |
| **Description** | Fix or document diff truncation (line 27) so it does not split a UTF-8 multi-byte character; truncate at rune boundaries or document the limitation. |
| **Acceptance** | `go test ./cli/internal/commitmsg/...` (if any). |

### T-WRN-04 – diff_test hardcoded messages

| Field | Value |
| ----- | ----- |
| **Issues** | #41 |
| **Files** | `cli/internal/diff/diff_test.go` |
| **Description** | Reduce reliance on hardcoded commit messages in TestHunks_integration_twoCommits so the test is less brittle (e.g. use git write or accept any two commits). |
| **Acceptance** | `go test ./cli/internal/diff/...` |

### T-WRN-05 – expand: nil guard and test expected line

| Field | Value |
| ----- | ----- |
| **Issues** | #42, #43 |
| **Files** | `cli/internal/expand/expand.go`, `cli/internal/expand/expand_test.go` |
| **Description** | In expand.go: add nil guard in function body extraction (line 104). In expand_test.go: fix the expected end line for test case 'no_count_implied' (line 30). |
| **Acceptance** | `go test ./cli/internal/expand/...` |

### T-WRN-06 – findings: nil guards and doc/test fixes

| Field | Value |
| ----- | ----- |
| **Issues** | #44, #45, #46, #47, #48 |
| **Files** | `cli/internal/findings/cursor_uri.go`, `cli/internal/findings/evidence_test.go`, `cli/internal/findings/fpkilllist.go`, `cli/internal/findings/id.go`, `cli/internal/findings/id_test.go` |
| **Description** | Add nil guards: cursor_uri.go list[i].Range (line 25); fpkilllist.go when input list is nil (line 44). In evidence_test.go: rename or deduplicate test case 'e_range_overlap_low_end_kept'. In id.go: align ShortID doc or behavior when len(id)<=ShortIDDisplayLen. In id_test.go: fix or document 'one_match_seven_chars' logic. |
| **Acceptance** | `go test ./cli/internal/findings/...` |

### T-WRN-07 – git: notes injection and tests

| Field | Value |
| ----- | ----- |
| **Issues** | #50, #51, #52, #53, #54, #55, #56, #57 |
| **Files** | `cli/internal/git/notes.go`, `cli/internal/git/notes_test.go`, `cli/internal/git/repo.go`, `cli/internal/git/revlist.go`, `cli/internal/git/revlist_test.go`, `cli/internal/git/worktree.go`, `cli/internal/git/worktree_test.go` |
| **Description** | notes.go: mitigate command injection in AddNote (line 25)—sanitize or use safe API. notes_test.go: avoid hardcoded timestamp that causes flakiness. repo.go: nil guard in UserIntent (94). revlist.go: nil guard in error handling (29). revlist_test: strengthen twoCommitsInRange (verify commit added, expected SHAs) and document or relax HEAD~2 assumption. worktree.go: nil guard in IsAncestor (64). worktree_test: document or fix TestCreate synchronization. |
| **Acceptance** | `go test ./cli/internal/git/...` |

### T-WRN-08 – history: race, filename, nil guards

| Field | Value |
| ----- | ----- |
| **Issues** | #58, #59, #61, #62 |
| **Files** | `cli/internal/history/append.go`, `cli/internal/history/append_test.go`, `cli/internal/history/suppression.go`, `cli/internal/history/suppression_test.go` |
| **Description** | append.go: fix or document race when checking file size and rotating (line 135). append_test.go: use actual archive naming pattern instead of hardcoded 'history.jsonl.1.gz'. suppression.go: nil guard in formatExample when f.Line is 0 (line 45). suppression_test.go: guard SuppressionExamples when maxRecords=0 (line 104). |
| **Acceptance** | `go test ./cli/internal/history/...` |

### T-WRN-09 – hunkid edge cases

| Field | Value |
| ----- | ----- |
| **Issues** | #63 |
| **Files** | `cli/internal/hunkid/hunkid.go` |
| **Description** | In StableFindingID (line 39), ensure rangeStart and rangeEnd validation correctly handles edge cases where one or both are zero. |
| **Acceptance** | `go test ./cli/internal/hunkid/...` |

### T-WRN-10 – ollama retry overflow

| Field | Value |
| ----- | ----- |
| **Issues** | #65 |
| **Files** | `cli/internal/ollama/client.go` |
| **Description** | Guard retry timeout calculation (line 73) against overflow for large retry attempts. |
| **Acceptance** | `go test ./cli/internal/ollama/...` |

### T-WRN-11 – rag: gitGrepSymbol nil and callgraph test cleanup

| Field | Value |
| ----- | ----- |
| **Issues** | #66, #67, #68, #69, #70, #71, #72, #73 |
| **Files** | `cli/internal/rag/go/callgraph.go`, `cli/internal/rag/go/callgraph_test.go`, `cli/internal/rag/go/resolver.go`, `cli/internal/rag/java/resolver.go`, `cli/internal/rag/js/resolver.go`, `cli/internal/rag/python/resolver.go`, `cli/internal/rag/swift/resolver.go` |
| **Description** | In all gitGrepSymbol call sites: guard against nil or type-assert failure when cmd.Output() returns an error (callgraph.go 50, js 67, python 104, swift 53). In go/resolver.go: improve regex for symbols at end of line or whitespace (135). In java/resolver.go: document or improve symbol regex for generics (44). In callgraph_test.go: add cleanup for temp file (16) and temp directory (37). |
| **Acceptance** | `go test ./cli/internal/rag/...` |

### T-WRN-12 – review: critic_test and suppressionBudget

| Field | Value |
| ----- | ----- |
| **Issues** | #75, #76 |
| **Files** | `cli/internal/review/critic_test.go`, `cli/internal/review/review.go` |
| **Description** | critic_test.go: ensure TestVerifyFinding_passesKeepAliveWhenSet validates KeepAlive correctly (line 170). review.go: handle or document negative suppressionBudget when baseNoSupp + DefaultResponseReserve exceeds contextLimit (line 107). |
| **Acceptance** | `go test ./cli/internal/review/...` |

### T-WRN-13 – rules: loader and FilterRules nil

| Field | Value |
| ----- | ----- |
| **Issues** | #77, #78 |
| **Files** | `cli/internal/rules/loader.go`, `cli/internal/rules/rules.go` |
| **Description** | loader.go: guard filepath.Rel when repoRoot is not a valid directory (54). rules.go: guard FilterRules when r.Globs is nil (107). |
| **Acceptance** | `go test ./cli/internal/rules/...` |

### T-WRN-14 – run.go timeout and doc

| Field | Value |
| ----- | ----- |
| **Issues** | #79, #121 |
| **Files** | `cli/internal/run/run.go` |
| **Description** | Ensure retry logic properly caps retry timeouts (e.g. to 30 min) when default is 15 min (line 104). Add documentation for the retry timeout cap. |
| **Acceptance** | `go test ./cli/internal/run/...`; comment or doc present for retry cap. |

### T-WRN-15 – scope_test comment

| Field | Value |
| ----- | ----- |
| **Issues** | #80 |
| **Files** | `cli/internal/scope/scope_test.go` |
| **Description** | Fix the misleading comment in TestPartition_semanticMatch (line 182) to match expected behavior. |
| **Acceptance** | `go test ./cli/internal/scope/...` |

### T-WRN-16 – session Save nil and session_test

| Field | Value |
| ----- | ----- |
| **Issues** | #82, #83 |
| **Files** | `cli/internal/session/session.go`, `cli/internal/session/session_test.go` |
| **Description** | session.go: guard Save when s is nil (line 103). session_test.go: guard comparison of DismissedIDs in TestSaveLoad_roundtrip (line 104). |
| **Acceptance** | `go test ./cli/internal/session/...` |

### T-WRN-17 – stats: division by zero and typo

| Field | Value |
| ----- | ----- |
| **Issues** | #84, #85, #86, #87, #88 |
| **Files** | `cli/internal/stats/energy.go`, `cli/internal/stats/energy_test.go`, `cli/internal/stats/gitai.go`, `cli/internal/stats/quality.go`, `cli/internal/stats/stats_test.go` |
| **Description** | energy.go: guard cloud cost when totalPrompt or totalCompletion is zero (43). energy_test.go: guard when watts is 0 (104). gitai.go: nil guard in parseGitAINote (59). quality.go: guard FalsePositiveRate division (63). stats_test.go: fix typo 'overriden_lines' to 'overridden_lines' (140). |
| **Acceptance** | `go test ./cli/internal/stats/...` |

### T-WRN-18 – version_test isolation

| Field | Value |
| ----- | ----- |
| **Issues** | #91 |
| **Files** | `cli/internal/version/version_test.go` |
| **Description** | Fix test setup so global state is isolated (line 13); avoid defer restoring globals in a way that makes the test order-dependent or fragile. |
| **Acceptance** | `go test ./cli/internal/version/...` |

### T-WRN-19 – Extension: cli.ts race and JSDoc

| Field | Value |
| ----- | ----- |
| **Issues** | #93, #94, #140, #141 |
| **Files** | `extension/src/cli.ts`, `extension/src/cli.test.ts` |
| **Description** | In cli.ts: fix or document race in spawnStetStream where stdoutBuffer may be accessed concurrently (59); add JSDoc for `args` in spawnStet (35) and spawnStetStream (59). In cli.test.ts: fix test so close callback is the one registered by spawnStet (40). |
| **Acceptance** | `cd extension && npm run compile && npm test` |

### T-WRN-20 – Extension: copyForChat and openFinding

| Field | Value |
| ----- | ----- |
| **Issues** | #95, #96, #102 |
| **Files** | `extension/src/copyForChat.ts`, `extension/src/copyForChat.test.ts`, `extension/src/openFinding.ts` |
| **Description** | copyForChat: fix line number in file fragment when both range and line are present (45). copyForChat.test: align test expectation (range start-end vs line number) (34). openFinding: fix selection handling when cursor_uri is valid but fragment is invalid (50). |
| **Acceptance** | `cd extension && npm test` |

### T-WRN-21 – Extension: findingsPanel, finishReview, extension race

| Field | Value |
| ----- | ----- |
| **Issues** | #97, #98, #99, #100, #101 |
| **Files** | `extension/src/extension.ts`, `extension/src/findingsPanel.ts`, `extension/src/findingsPanel.test.ts`, `extension/src/finishReview.ts`, `extension/src/finishReview.test.ts` |
| **Description** | extension.ts: fix or document race in findings accumulation (104). findingsPanel.ts: guard tooltip appendMarkdown calls (56). findingsPanel.test: avoid toMatchObject masking property mismatches or tighten assertion (105). finishReview.ts: ensure provider.clear() is not left skipped by exception (24). finishReview.test: fix expectations for 'does not clear panel on exit code 2' (36). |
| **Acceptance** | `cd extension && npm test` |

### T-WRN-22 – Install scripts and Makefile

| Field | Value |
| ----- | ----- |
| **Issues** | #103, #104, #35 |
| **Files** | `install.ps1`, `install.sh`, `Makefile` |
| **Description** | install.ps1: fix race in temporary file handling (56). install.sh: fix race in checksum verification (55). Makefile: fix or document race in release target with parallel GOOS/GOARCH builds (23). |
| **Acceptance** | Scripts run without error; `make release` (or equivalent) still works. |

### T-WRN-23 – README and docs roadmap

| Field | Value |
| ----- | ----- |
| **Issues** | #36, #92 |
| **Files** | `README.md`, `docs/roadmap.md` |
| **Description** | README: fix inconsistent code block formatting in installation instructions (34). roadmap.md: make default critic model documentation consistent (125). |
| **Acceptance** | No functional break; formatting and wording consistent. |

### T-WRN-24 – run.go critic loop doc

| Field | Value |
| ----- | ----- |
| **Issues** | #105 |
| **Files** | `cli/internal/run/run.go` |
| **Description** | Document that the critic loop (lines 378–390) performs O(N) LLM calls per batch and that batching is not used because the API does not support it; or add batching if the API allows. |
| **Acceptance** | `go test ./cli/internal/run/...`; comment or doc present. |

---

## INFO tasks

### T-INF-01 – Go package and file comments

| Field | Value |
| ----- | ----- |
| **Issues** | #110, #112, #113, #114, #116, #120, #122, #123, #124 |
| **Files** | `cli/internal/benchmark/benchmark_test.go`, `cli/internal/findings/cursor_uri_test.go`, `cli/internal/findings/fpkilllist_test.go`, `cli/internal/git/repo_test.go`, `cli/internal/ollama/client_test.go`, `cli/internal/rag/rag.go`, `cli/internal/scope/scope.go`, `cli/internal/review/parse_test.go`, `cli/internal/stats/volume.go` |
| **Description** | Add or fix package documentation comment where missing. In rag.go Definition struct, add periods at end of field comments. In scope.go fix package comment if it contains a typo ('stet' vs 'stet'). |
| **Acceptance** | `cd cli && go build ./...`; no new vet/lint issues. |

### T-INF-02 – Go test names and go.mod

| Field | Value |
| ----- | ----- |
| **Issues** | #111, #117, #145 |
| **Files** | `cli/internal/config/config_test.go`, `cli/internal/prompt/prompt_test.go`, `go.mod` |
| **Description** | Use more descriptive test function names in config_test (18) and prompt_test (14). In go.mod ensure module declaration is on its own line without leading blank lines. |
| **Acceptance** | `go test ./cli/internal/config/... ./cli/internal/prompt/...`; `go build ./cli/...` with current go.mod. |

### T-INF-03 – Docs: titles, headers, typo

| Field | Value |
| ----- | ----- |
| **Issues** | #129, #130, #131, #132, #133, #137, #138 |
| **Files** | `docs/PRD-Refine.md`, `docs/cursor-orchestrator-prd.md`, `docs/efficacy-tests.md`, `docs/implementation-plan-refine.md`, `docs/implementation-plan.md`, `docs/PRD.md`, `docs/cli-extension-contract.md` |
| **Description** | Add missing title (PRD-Refine, cursor-orchestrator-prd). Add missing header (efficacy-tests). Add version/changelog note to implementation-plan-refine if appropriate. Fix typo in 'stet stats energy' in implementation-plan (245). Add accessibility category to findings output format in PRD (187). In cli-extension-contract add brief guidance on accessibility use/implementation (35). |
| **Acceptance** | Markdown valid; no code behavior change. |

### T-INF-04 – Extension: JSDoc, header, and test style

| Field | Value |
| ----- | ----- |
| **Issues** | #139, #142, #143, #144 |
| **Files** | `extension/package.json`, `extension/src/openFinding.test.ts`, `extension/tsconfig.json`, `extension/vitest.config.ts` |
| **Description** | package.json: align package name with branding if needed. openFinding.test: simplify Position class to direct property declarations (10). vitest.config.ts: add file header comment. tsconfig.json: no change required (info only); skip or add comment. |
| **Acceptance** | `cd extension && npm run compile && npm test` |

### T-INF-05 – prompt.go redundant condition

| Field | Value |
| ----- | ----- |
| **Issues** | #147 |
| **Files** | `cli/internal/prompt/prompt.go` |
| **Description** | In UserPromptSearchReplace (line 404), remove redundant len(line)<1 check if len(line)==0 is already handled, or document why both are needed. |
| **Acceptance** | `go test ./cli/internal/prompt/...` |

### T-INF-06 – Doc files (optional)

| Field | Value |
| ----- | ----- |
| **Issues** | #106–#109, #125–#128, #134–#136 |
| **Files** | Various under `docs/` and root (e.g. .devcontainer, .gitignore, AGENTS.md, LICENSE) |
| **Description** | Optional: add or adjust doc metadata (titles, changelog, completeness) for calibration-fp-aggregation, code-review-research-topics, context-enrichment-research, defect-focused-review-plan, phase-3-remediation-plan, review-process-internals, review-quality. No code changes required. |
| **Acceptance** | None required; doc-only. |

---

## Full verification

After implementing tasks, run from the repository root:

```bash
cd cli && go build ./... && go test ./... -count=1
cd ../extension && npm run compile && npm test
```

Resolve any failing tests or build errors before considering remediation complete.
