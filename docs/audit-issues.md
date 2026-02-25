# Audit issues – enumerated list

This document enumerates all issues raised by the **stet code review** (`stet list`) and the **brutal audit** (Go + JavaScript/TypeScript audit workflows). Use it to track remediation. Items are grouped by severity and area; verification notes indicate where the finding was checked against the codebase.

**How to use:** Address ERRORs first, then WARNINGs, then INFO. Mark items done in your tracker or remove them from this list as fixed.

---

## Verification notes (stet vs codebase)

These findings were spot-checked; adjust as you fix.

| Finding | Location | Verification |
| --------- | ---------- | -------------- |
| erruser.Error() nil pointer | erruser.go:18 | **False positive:** Code has `if e == nil { return "" }` at 16–18. |
| Missing 'accessibility' in CATEGORIES | extension/src/contract.ts:30 | **False positive:** `accessibility` is present at line 26 in `CATEGORIES`. |
| Missing closing backtick in parse_test | diff/parse_test.go:24 | **Verify:** Current code uses a proper backtick string (lines 31–41). Dismiss if no longer applicable. |
| Missing shebang in check-coverage.sh | scripts/check-coverage.sh:1 | **False positive:** File has `#!/usr/bin/env bash` on line 1. |
| Default timeout 15 vs comment 5 min | config.go:63 | **Verify:** Constant is 15 min at line 123. Search codebase for any comment that says "5 minutes" and fix. |
| KeepAlive when opts.KeepAlive is nil | critic.go:107 | **False positive:** When `opts != nil && opts.KeepAlive == nil`, code leaves `genOpts.KeepAlive = 0`. Correct. |
| Import path should be github.com/stet/... | Multiple | **Intentional:** Module is `stet` in go.mod; imports `stet/cli/internal/...` are correct. Only change if module path is updated. |
| Package name 'goresolver' vs directory 'go' | rag/go/*.go | **Intentional:** Directory is `go` (reserved word); package `goresolver` is used by convention. |
| Package name 'stats' vs directory quality_test.go | stats/quality_test.go | **False positive:** Test file in package `stats` is valid; directory is `stats`. |

---

## ERROR (fix first)

| # | ID / Ref | File | Line | Description | Source |
| --- | --- | --- | --- | --- | --- |
| 1 | c5f9448 | cli/internal/diff/parse_test.go | 24 | Missing closing backtick for diff string literal in TestParseUnifiedDiff_singleFileSingleHunk. Verify current code. | stet |
| 2 | c67f4fa | cli/internal/config/config.go | 63 | Default timeout is 15 minutes; ensure no comment says "5 minutes". | stet |
| 3 | 1600cc0 | cli/internal/erruser/erruser.go | 18 | Potential nil pointer dereference in Error() – verified false positive (nil check present). | stet |
| 4 | 7bedc8e | cli/internal/erruser/erruser_test.go | 10 | Missing import for 'errors' package in test file. | stet |
| 5 | 10b4648 | cli/internal/findings/abstention.go | 17 | Missing newline at end of function body. | stet |
| 6 | 87aa900 | cli/internal/findings/abstention_test.go | 10 | Missing import for 'testing' package in test file. | stet |
| 7 | 3259047 | cli/internal/findings/evidence.go | 35 | Logic error in range overlap check: edge cases where ranges touch but don't overlap. Clarify or fix condition/comment. | stet |
| 8 | 9e9a324 | cli/internal/findings/strictness.go | 47 | Logic error in default case of switch: returns true for applyFP; default is unreachable – remove or fix. | stet + audit |
| 9 | d545a61 | cli/internal/findings/strictness_test.go | 10 | Missing import for 'strings' package. | stet |
| 10 | 6d3d13d | cli/internal/history/schema_test.go | 1 | Import path 'stet/cli/internal/findings' – stet suggests github.com/...; project uses module `stet`. See verification. | stet |
| 11 | 3a7ceca | cli/internal/hunkid/hunkid_test.go | 15 | Missing import for 'testing' package in test file. | stet |
| 12 | f7bebb6 | cli/internal/minify/minify.go | 33 | Potential panic on empty line access when line length is 1. Verify: len(line)>=1 before line[0:1]. | stet |
| 13 | 1715513 | cli/internal/rag/go/callgraph_test.go | 1 | Package name 'goresolver' vs directory – intentional (directory `go` is keyword). | stet |
| 14 | e0f5164 | cli/internal/rag/go/resolver_test.go | 1 | Same as above. | stet |
| 15 | e0923cb | cli/internal/rag/java/resolver_test.go | 1 | Import path 'stet/cli/internal/rag' – see verification (module is stet). | stet |
| 16 | 938082f | cli/internal/rag/js/resolver_test.go | 1 | Import path – see verification. | stet |
| 17 | f991bd2 | cli/internal/rag/python/resolver_test.go | 1 | Package name 'python' does not match directory name – directory is python; package python is valid. | stet |
| 18 | 9c1e0d8 | cli/internal/rag/rag_test.go | 22 | TestResolveSymbols_unknownExtension_returnsNil expects nil return but ResolveSymbols may return empty slice. Align test or implementation. | stet + audit |
| 19 | 58a3604 | cli/internal/rag/swift/resolver_test.go | 1 | Package import path incorrect – see verification. | stet |
| 20 | 801ca78 | cli/internal/review/critic.go | 107 | Incorrect assignment of KeepAlive when opts.KeepAlive is nil – verified false positive. | stet |
| 21 | c1dfb65 | cli/internal/rules/rules_test.go | 15 | Test function name TestParseMDC_globsString_contentParsed should match pattern (e.g. TestParseMDC_globsString). | stet |
| 22 | 3363a73 | cli/internal/session/lock_windows.go | 45 | Potential nil pointer dereference in error handling path. | stet |
| 23 | e704b54 | cli/internal/stats/gitai_test.go | 1 | Import path 'stet/cli/internal/git' – see verification. | stet |
| 24 | ce7024b | cli/internal/stats/quality_test.go | 1 | Package name 'stats' vs directory – verified false positive (directory is stats). | stet |
| 25 | 73912a6 | cli/internal/tokens/estimate.go | 47 | Potential integer overflow in token calculation when promptTokens near math.MaxInt. | stet |
| 26 | 8d0e108 | cli/internal/trace/trace_test.go | 10 | TestNew_nilWriter_returnsTracer: logic error – checks tr==nil then t.Fatal; may prevent other tests from running. | stet |
| 27 | 70f27ff | cli/internal/version/version.go | 14 | Missing newline at end of function body. | stet |
| 28 | 2615f4e | cli/internal/findings/finding.go | 31 | Missing newline at end of file. | stet |
| 29 | 8df7ef7 | cli/internal/findings/finding_test.go | 109 | TestFindingValidate case 'valid_accessibility' uses CategoryAccessibility not defined in test file. | stet |
| 30 | acf67f4 | cli/internal/findings/validation.go | 32 | Inconsistent normalization: Normalize sets invalid category to CategoryBug; document that Validate then accepts it (CategoryBug in validCategories). | stet + audit |
| 31 | d1010a1 | cli/internal/review/parse.go | 58 | Single-object fallback path does not normalize invalid categories before validation. | stet |
| 32 | 7b101aa | cli/internal/review/parse_test.go | 10 | Missing import for 'strings' package. | stet |
| 33 | e09e8ab | extension/src/contract.ts | 30 | Missing 'accessibility' in CATEGORIES – verified false positive (present at line 26). | stet |
| 34 | a3d2738 | .devcontainer/Dockerfile | 1 | Untrusted script installation from external sources without checksum verification. | stet |

---

## WARNING (fix next)

| # | ID / Ref | File | Line | Description | Source |
| --- | --- | --- | --- | --- | --- |
| 35 | c6bfa66 | Makefile | 23 | Potential race condition in release target (parallel GOOS/GOARCH builds). | stet |
| 36 | 333ecc7 | README.md | 34 | Inconsistent code block formatting in installation instructions. | stet |
| 37 | f10a431 | cli/cmd/stet/main.go | 1027 | Potential nil pointer dereference in commitmsg.Suggest call. | stet |
| 38 | 4746037 | cli/cmd/stet/main.go | 1029 | Possible incorrect NumCtx assignment when cfg.NumCtx is 0. | stet |
| 39 | 0f2d926 | cli/internal/benchmark/benchmark.go | 70 | Potential division by zero in EvalRateTPS calculation. | stet |
| 40 | c08072d | cli/internal/commitmsg/commitmsg.go | 27 | Potential logic error in diff truncation: may split UTF-8 multi-byte character. | stet |
| 41 | fcb3769 | cli/internal/diff/diff_test.go | 107 | TestHunks_integration_twoCommits may fail due to hardcoded commit messages. | stet |
| 42 | 4f0605e | cli/internal/expand/expand.go | 104 | Potential nil pointer dereference in function body extraction. | stet |
| 43 | bbdb852 | cli/internal/expand/expand_test.go | 30 | TestHunkLineRange case 'no_count_implied' has incorrect expected end line. | stet |
| 44 | 697bf03 | cli/internal/findings/cursor_uri.go | 25 | Potential nil pointer dereference when accessing list[i].Range. | stet |
| 45 | c96b992 | cli/internal/findings/evidence_test.go | 107 | Test case name 'e_range_overlap_low_end_kept' duplicated with 'e_range_overlapping_hunk_kept'. | stet |
| 46 | 3a8b824 | cli/internal/findings/fpkilllist.go | 44 | Potential nil pointer dereference in FilterFPKillList when input list is nil. | stet |
| 47 | d8e1a56 | cli/internal/findings/id.go | 20 | ShortID: returns full ID when len(id)<=ShortIDDisplayLen; doc says first ShortIDDisplayLen chars. Align doc or behavior. | stet |
| 48 | 5c75870 | cli/internal/findings/id_test.go | 34 | TestResolveFindingIDByPrefix 'one_match_seven_chars' potential logic error. | stet |
| 49 | 05f2b29 | cli/internal/findings/validation_test.go | 67 | TestFindingNormalize_nilNoop calls Normalize on nil pointer (may panic). | stet |
| 50 | ce25ff3 | cli/internal/git/notes.go | 25 | Potential command injection vulnerability in AddNote. | stet |
| 51 | fa99d94 | cli/internal/git/notes_test.go | 10 | Test uses hardcoded timestamp; may cause flaky tests. | stet |
| 52 | 9d54ebd | cli/internal/git/repo.go | 94 | Potential nil pointer dereference in UserIntent. | stet |
| 53 | 331db8c | cli/internal/git/revlist.go | 29 | Potential nil pointer dereference in error handling when checking exit code. | stet |
| 54 | 4df6415 | cli/internal/git/revlist_test.go | 13 | TestRevList_twoCommitsInRange doesn't verify commit added or expected SHAs. | stet |
| 55 | 991a93b | cli/internal/git/revlist_test.go | 20 | Test assumes HEAD~2 always exists and contains exactly 2 commits. | stet |
| 56 | 886ed92 | cli/internal/git/worktree.go | 64 | Potential nil pointer dereference in IsAncestor when handling exit code. | stet |
| 57 | f080260 | cli/internal/git/worktree_test.go | 106 | TestCreate may fail if worktree creation not fully synchronized. | stet |
| 58 | 6d9cbfe | cli/internal/history/append.go | 135 | Potential race condition in Append when checking file size and rotating. | stet |
| 59 | 131c525 | cli/internal/history/append_test.go | 107 | Test uses hardcoded archive filename 'history.jsonl.1.gz' – may not match actual naming. | stet |
| 60 | e6f794d | cli/internal/history/schema.go | 26 | Missing newline at end of file. | stet |
| 61 | 7727d9b | cli/internal/history/suppression.go | 45 | Potential nil pointer dereference in formatExample when f.Line is 0 and f.File not empty. | stet |
| 62 | 629af54 | cli/internal/history/suppression_test.go | 104 | Potential nil pointer dereference in SuppressionExamples when maxRecords=0. | stet |
| 63 | eb75773 | cli/internal/hunkid/hunkid.go | 39 | StableFindingID: rangeStart/rangeEnd validation may not handle zero edge cases. | stet |
| 64 | 182b60b | cli/internal/minify/minify_test.go | 27 | Test case 'header plus lines with leading whitespace' has incorrect expected output. | stet |
| 65 | 0627042 | cli/internal/ollama/client.go | 73 | Retry timeout calculation may overflow for large retry attempts. | stet |
| 66 | 96fb123 | cli/internal/rag/go/callgraph.go | 50 | Potential nil pointer dereference in gitGrepSymbol call. | stet |
| 67 | 5a4f1a0 | cli/internal/rag/go/callgraph_test.go | 16 | Test creates temp file but doesn't clean up directory structure. | stet |
| 68 | 9680911 | cli/internal/rag/go/callgraph_test.go | 37 | Test creates temp directory but doesn't clean it up. | stet |
| 69 | d772991 | cli/internal/rag/go/resolver.go | 135 | Regex for git grep may not handle symbols at end of line or whitespace edge cases. | stet |
| 70 | de053c7 | cli/internal/rag/java/resolver.go | 44 | Symbol extraction regex may not handle all Java identifier patterns (generics, etc.). | stet |
| 71 | 765955d | cli/internal/rag/js/resolver.go | 67 | Potential nil pointer dereference in gitGrepSymbol when cmd.Output() returns error and e.ExitCode() called. | stet |
| 72 | 1634433 | cli/internal/rag/python/resolver.go | 104 | Potential nil pointer dereference in gitGrepSymbol when error is not *exec.ExitError. | stet |
| 73 | 6891619 | cli/internal/rag/swift/resolver.go | 53 | Same as above. | stet |
| 74 | fe8f50b | cli/internal/review/critic.go | 107 | Potential nil pointer dereference when opts.KeepAlive is nil but opts not nil – verified false positive. | stet |
| 75 | 94813b0 | cli/internal/review/critic_test.go | 170 | TestVerifyFinding_passesKeepAliveWhenSet may not properly validate KeepAlive (nil deref). | stet |
| 76 | bb75663 | cli/internal/review/review.go | 107 | suppressionBudget may go negative; suppression examples may not be added. Document or fix. | stet + audit |
| 77 | aad0ff9 | cli/internal/rules/loader.go | 54 | Potential nil pointer dereference in filepath.Rel when repoRoot invalid. | stet |
| 78 | 6a5b7d5 | cli/internal/rules/rules.go | 107 | Potential nil pointer dereference in FilterRules when r.Globs is nil. | stet |
| 79 | 672f8cb | cli/internal/run/run.go | 104 | Timeout handling: default 15 min; retry logic may not cap retry timeouts to 30 min. | stet |
| 80 | 378e931 | cli/internal/scope/scope_test.go | 182 | TestPartition_semanticMatch has misleading comment about expected behavior. | stet |
| 81 | f92e7ab | cli/internal/session/lock_unix.go | 25 | Resource leak: file descriptor may not be closed if syscall.Flock fails. | stet |
| 82 | 364e1f9 | cli/internal/session/session.go | 103 | Potential nil pointer dereference in Save when s is nil. | stet |
| 83 | 961fb1e | cli/internal/session/session_test.go | 104 | Potential nil pointer dereference in TestSaveLoad_roundtrip when comparing DismissedIDs. | stet |
| 84 | 5aefdce | cli/internal/stats/energy.go | 43 | Potential division by zero in cloud cost when totalPrompt or totalCompletion is zero. | stet |
| 85 | d3e9c87 | cli/internal/stats/energy_test.go | 104 | Potential division by zero when watts parameter is 0. | stet |
| 86 | 9df262e | cli/internal/stats/gitai.go | 59 | Potential nil pointer dereference in parseGitAINote. | stet |
| 87 | 0cef3b3 | cli/internal/stats/quality.go | 63 | Potential division by zero in FalsePositiveRate. | stet |
| 88 | be3f0cb | cli/internal/stats/stats_test.go | 140 | Test uses 'overriden_lines' – typo for 'overridden_lines'. | stet |
| 89 | ad8f020 | cli/internal/tokens/estimate_test.go | 74 | Test case 'overflow_returns_warning' may cause integer overflow in calculation. | stet |
| 90 | 1f0cb39 | cli/internal/trace/trace.go | 25 | Potential nil pointer dereference in Section method. | stet |
| 91 | 6878baa | cli/internal/version/version_test.go | 13 | Test defer restores globals; test modifies global state without proper isolation. | stet |
| 92 | abe5f16 | docs/roadmap.md | 125 | Inconsistent documentation of default critic model in multiple locations. | stet |
| 93 | 4c11ec9 | extension/src/cli.test.ts | 40 | Close callback from mock calls may not be the one registered by spawnStet. | stet |
| 94 | e86f7fe | extension/src/cli.ts | 59 | Potential race condition in spawnStetStream: stdoutBuffer accessed concurrently. | stet + audit |
| 95 | fd7e969 | extension/src/copyForChat.test.ts | 34 | Test expects range start-end in link text but uses line number. | stet |
| 96 | 58d2dd3 | extension/src/copyForChat.ts | 45 | Potential incorrect line number in file fragment when range and line both present. | stet |
| 97 | 19e12ea | extension/src/extension.ts | 104 | Potential race condition in findings accumulation. | stet |
| 98 | beb829c | extension/src/findingsPanel.test.ts | 105 | toMatchObject may mask exact property mismatches in command arguments. | stet |
| 99 | cc8ce4c | extension/src/findingsPanel.ts | 56 | Potential nil pointer dereference in tooltip appendMarkdown calls. | stet |
| 100 | e2af54b | extension/src/finishReview.test.ts | 36 | Test case 'does not clear panel on exit code 2' has inconsistent expectations. | stet |
| 101 | 44edeb1 | extension/src/finishReview.ts | 24 | Race in error handling: provider.clear() may be interrupted by exception. | stet |
| 102 | 8cfe09e | extension/src/openFinding.ts | 50 | Potential logic error in selection when cursor_uri valid but fragment invalid. | stet |
| 103 | 6455324 | install.ps1 | 56 | Potential race condition in temporary file handling. | stet |
| 104 | 177939a | install.sh | 55 | Potential race condition in checksum verification logic. | stet |
| 105 | (audit) | cli/internal/run/run.go | 378–390 | Critic loop: O(N) LLM calls per batch; document or add batching if API supports it. | audit |

---

## INFO (improvements and docs)

| # | ID / Ref | File | Line | Description | Source |
| --- | --- | --- | --- | --- | --- |
| 106 | 40a988d | .devcontainer/devcontainer.json | 1 | Devcontainer updated with explicit Go, Node, Git feature versions. | stet |
| 107 | c2165ba | .gitignore | 1 | .gitignore updated with Go, TypeScript, dev entries. | stet |
| 108 | 6245fed | AGENTS.md | 1 | AGENTS.md new or major update with guidelines. | stet |
| 109 | dab2ad7 | LICENSE | 1 | License updated with new copyright year. | stet |
| 110 | c06ae96 | cli/internal/benchmark/benchmark_test.go | 1 | Package comment missing. | stet |
| 111 | 0fb804d | cli/internal/config/config_test.go | 18 | Test function name could be more descriptive. | stet |
| 112 | 39ec048 | cli/internal/findings/cursor_uri_test.go | 1 | Missing package documentation comment. | stet |
| 113 | b94f74f | cli/internal/findings/fpkilllist_test.go | 1 | Missing package documentation comment. | stet |
| 114 | e042ed3 | cli/internal/git/repo_test.go | 10 | Missing package documentation comment. | stet |
| 115 | 435af08 | cli/internal/git/revlist_test.go | 79 | TestRevList_emptyRefs doesn't verify error message content. | stet |
| 116 | 17f7e43 | cli/internal/ollama/client_test.go | 1 | Missing package documentation comment. | stet |
| 117 | 35bdbe4 | cli/internal/prompt/prompt_test.go | 14 | Test function name could be more descriptive. | stet |
| 118 | 063ee2b | cli/internal/rag/go/callgraph_test.go | 57 | Use multi-line string literal with backticks for source code. | stet |
| 119 | be55687 | cli/internal/rag/go/callgraph_test.go | 89 | Same as above. | stet |
| 120 | a5337e8 | cli/internal/rag/rag.go | 17 | Definition struct field comments missing periods at end. | stet |
| 121 | 2d99eb1 | cli/internal/run/run.go | 104 | Missing documentation for retry timeout cap implementation. | stet |
| 122 | 2d3017c | cli/internal/scope/scope.go | 1 | Package comment uses 'stet' instead of 'stet' – likely typo. | stet |
| 123 | 509036c | cli/internal/review/parse_test.go | 1 | Missing package documentation comment. | stet |
| 124 | 7d8e01d | cli/internal/stats/volume.go | 1 | Package comment missing period at end. | stet |
| 125 | 31802d9 | docs/calibration-fp-aggregation.md | 1 | Doc updated with research content. | stet |
| 126 | 2f5c855 | docs/code-review-research-topics.md | 1 | Research summary, no functional code. | stet |
| 127 | 842ee9b | docs/context-enrichment-research.md | 1 | Context enrichment research update. | stet |
| 128 | 6e383be | docs/defect-focused-review-plan.md | 1 | Research plan, no code defects. | stet |
| 129 | 18c081c | docs/PRD-Refine.md | 1 | Missing title in markdown. | stet |
| 130 | 60e2485 | docs/cursor-orchestrator-prd.md | 1 | Missing title in markdown. | stet |
| 131 | 110604b | docs/efficacy-tests.md | 1 | Missing header. | stet |
| 132 | 6cbe8f5 | docs/implementation-plan-refine.md | 1 | Lacks version control or changelog entry. | stet |
| 133 | 55ebbca | docs/implementation-plan.md | 245 | Typo in 'stet stats energy' command description. | stet |
| 134 | 0b93c9d | docs/phase-3-remediation-plan.md | 1 | Doc updated with remediation plan. | stet |
| 135 | d25adc1 | docs/review-process-internals.md | 1 | Document incomplete or truncated. | stet |
| 136 | 8bf7f47 | docs/review-quality.md | 1 | Template or placeholder with examples. | stet |
| 137 | e9eacb8 | docs/PRD.md | 187 | Missing accessibility category in findings output format documentation. | stet |
| 138 | e0aaad2 | docs/cli-extension-contract.md | 35 | Accessibility mentioned for backward compatibility; no guidance on use/implementation. | stet |
| 139 | 1dbdacf | extension/package.json | 1 | Package name 'stet' and branding consistency. | stet |
| 140 | b54b0c7 | extension/src/cli.ts | 59 | Missing JSDoc parameter description for 'args' in spawnStetStream. | stet |
| 141 | bd5a029 | extension/src/cli.ts | 35 | Missing JSDoc parameter description for 'args' in spawnStet. | stet |
| 142 | 3eb0bf7 | extension/src/openFinding.test.ts | 10 | Position class could use direct property declarations. | stet |
| 143 | ab0913a | extension/tsconfig.json | 1 | tsconfig added with strict type checking. | stet |
| 144 | 64c1922 | extension/vitest.config.ts | 1 | Missing file header comment. | stet |
| 145 | 6898b80 | go.mod | 1 | Module declaration on its own line without leading blank lines. | stet |
| 146 | cbac595 | scripts/check-coverage.sh | 1 | Missing shebang – verified false positive (shebang present). | stet |
| 147 | 854cc17 | cli/internal/prompt/prompt.go | 404 | UserPromptSearchReplace: len(line)<1 redundant if len(line)==0 already checked. | stet |

---

## Summary counts

| Severity | Count |
| ---------- | ------- |
| ERROR | 34 |
| WARNING | 71 |
| INFO | 42 |
| **Total** | **147** |

After applying verification notes, treat several ERRORs as false positives or intentional; fix remaining ERRORs and high-priority WARNINGs first, then run `go build ./...`, `go test ./...`, and `npm run compile` / `npm test` to confirm.
