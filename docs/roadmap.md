# Stet Future Roadmap: Beyond Phase 7

**Status:** Draft / Research

**Context:** This document outlines the feature roadmap for stet after the core "Defect-Focused" implementation (Phase 6) and "Polish" (Phase 7) are complete. Roadmap phases below (Ecosystem, Adaptive, Deep Context) are thematic and distinct from implementation plan phases 0–9.

**Objective:** Evolve stet from a "local CLI tool" into a "universal review agent" that integrates with AI IDEs and learns from user behavior.

### Design principles

- **Precision-focused design:** Stet defaults to fewer, high-confidence, actionable findings (abstention, FP kill list, prompt shadowing, optimizer). Some false positives are expected; industry benchmarks for AI code review are roughly 5–15% FP rate, with precision-focused tools often at 5–8%. Stet aims for the lower end via context, filters, and feedback; monitor and tune (strictness presets, `stet optimize`, dismiss reasons) to keep noise acceptable.
- **Human-in-the-loop:** Treat stet output as first-pass review; the human makes the final call. Dismiss reasons and history improve future runs. This aligns with best practice: AI complements, not replaces, human review.
- **Context-aware review:** Git intent, hunk expansion, RAG-lite, and (roadmap) cross-file impact are core FP-reduction strategies—broader context reduces superficial or irrelevant flags.

### Already implemented (context)

The following exist today; this roadmap only lists work not yet implemented.

- **CLI:** `stet start`, `run`, `rerun`, `finish`, `status`, `list`, `dismiss`, `optimize`, `doctor`, `cleanup`.
- **Session and findings:** Active findings (Findings minus DismissedIDs), JSON/human output, stable finding IDs.
- **History:** Append to `.review/history.jsonl` on dismiss (with reasons) and on finish; schema supports optimizer.
- **RAG-lite:** Symbol definitions injected per language (Go, JS/TS, Python, Swift, Java); config `rag_symbol_max_*`.
- **Hunk expansion:** `expand.ExpandHunk` for enclosing function (Go) and N-line fallback.
- **Optimizer:** `stet optimize` writes `system_prompt_optimized.txt`; no suggested config output yet.
- **Strictness and nitpicky:** Presets and config/env overrides.

### Planned elsewhere

- **Impact reporting** (`stet stats` volume, quality, energy) is specified in [implementation-plan.md](implementation-plan.md) Phase 9 (sub-phases 9.1–9.8). Implement from that document.

---

## Phase 8: The "Ecosystem" Release (MCP & Integration)

**Goal:** Transform stet from a standalone tool into a service that AI Editors (Cursor, Windsurf, Claude) can consume directly.

### 8.1 Feature: Stet as an MCP Server

- **Status:** Not started.
- **Goal:** Expose stet via the Model Context Protocol so IDEs can trigger reviews and read findings without manual CLI execution.
- **Entry points:** New binary (e.g. `cmd/stet-mcp/`) or subcommand (e.g. `stet mcp` stdio). If subcommand: register with `rootCmd.AddCommand(newMcpCmd())` in [cli/cmd/stet/main.go](cli/cmd/stet/main.go) (around line 170). Document the chosen entry point.
- **Inputs:** MCP JSON-RPC over stdio. Tool `run_review(scope: string)` receives scope (e.g. staged, commit, branch). Tool `get_findings(min_confidence: float)` receives optional confidence threshold. State dir from env `STET_STATE_DIR` or default `.review` relative to repo root.
- **Outputs:** Resource `stet://latest_report` returns last review JSON (session + findings). Tools return JSON per MCP spec (e.g. findings array). Transport: stdio JSON-RPC; specify MCP spec version in docs.
- **Code to reuse:** [session.Load](cli/internal/session/session.go)(stateDir); pattern for active findings and writing findings JSON in [cli/cmd/stet/main.go](cli/cmd/stet/main.go) (e.g. `activeFindings`, `writeFindingsJSON`). [run](cli/internal/run/run.go) package for triggering review (Start/Run). Repo root from cwd via [git.RepoRoot](cli/internal/git).
- **Implementation chunks:**
  1. MCP server skeleton: stdio transport, JSON-RPC request/response dispatch. Unit test with canned stdin/stdout.
  2. Resource `stet://latest_report`: read session and findings from state dir; return JSON. Test: write fixture session, assert resource content.
  3. Tool `run_review(scope)`: invoke existing start/run flow (or subprocess of stet CLI). Test: mock or integration with dry-run.
  4. Tool `get_findings(min_confidence)`: load session, filter findings by confidence (see [findings.Finding](cli/internal/findings/finding.go) `Confidence`), return JSON. Test: fixture session, assert filtering.
  5. Document MCP capability and IDE integration (Cursor, etc.).
- **Config and env:** `STET_STATE_DIR` for state directory when running as MCP process. No new config file keys required for minimal version.
- **Acceptance criteria:** An MCP client can connect via stdio, request `stet://latest_report` and get valid JSON; call `run_review(scope)` and receive review result; call `get_findings(min_confidence)` and receive filtered findings.
- **Tests:** New and changed code must meet project coverage: 77% project, 72% per file (see [AGENTS.md](../AGENTS.md)).

### 8.2 Feature: Hybrid Linter Relay

- **Status:** Not started.
- **Goal:** Run a fast local linter on changed files and inject linter output into the review prompt so the LLM can explain and suggest fixes for syntax/static issues.
- **Entry points:** No new CLI command. Integrate into the review pipeline. Call sites: where the system or user prompt is built — [cli/internal/review/review.go](cli/internal/review/review.go) (ReviewHunk) and [cli/internal/run/run.go](cli/internal/run/run.go) (where hunks are iterated). Run linter per changed file (or per language) before or at start of review.
- **Inputs:** Changed files from diff (repo root + list of file paths). Config: linter command per language (e.g. Go → `golangci-lint`, JS/TS → `eslint`), or empty to disable. Repo root from [git.RepoRoot](cli/internal/git).
- **Outputs:** Linter output (file, line, code, message) injected into the prompt as a "Static analyzer reported: …" block (user or system prompt). No change to [findings.Finding](cli/internal/findings/finding.go) schema unless desired.
- **Code to reuse:** Diff pipeline for changed files (e.g. [cli/internal/diff](cli/internal/diff)); [cli/internal/prompt](cli/internal/prompt/prompt.go) — add `AppendLinterFindings(systemOrUserPrompt string, linterResults []LinterResult) string` or equivalent. [review.ReviewHunk](cli/internal/review/review.go) and run loop in [run.Run](cli/internal/run/run.go) to run linter and call the new append function.
- **Implementation chunks:**
  1. Config for linter commands per language (e.g. `linter_go`, `linter_js` in [config.Config](cli/internal/config/config.go) or env `STET_LINTER_GO`, `STET_LINTER_JS`). Load in config and pass to review path.
  2. Run linter on changed files (by language from extension); parse stdout into (file, line, code, message). Prefer existing linter output formats (e.g. golangci-lint JSON, eslint JSON). Unit test: parse canned linter output.
  3. Append linter block to prompt in review path (e.g. new function in prompt package, called from review.go after system prompt load). Integration test: mock linter output, assert prompt contains the block.
  4. Document prompt shape and config keys.
- **Config and env:** e.g. `[linter]` section or keys `linter_go`, `linter_js` in [config.Config](cli/internal/config/config.go); or env `STET_LINTER_GO`, `STET_LINTER_JS`. Empty or missing = disabled.
- **Acceptance criteria:** With linter enabled and a changed file that has a linter error, the review prompt includes a "Static analyzer reported: …" section and the model can reference it in findings or explanations.
- **Tests:** New and changed code must meet project coverage: 77% project, 72% per file (see [AGENTS.md](../AGENTS.md)).

### 8.3 Feature: Targeted Fix from Active Findings (`stet fix` / `stet refine`)

- **Status:** Not started.
- **Goal:** Use active findings (after user dismisses false positives) as input to a smaller local LLM to generate targeted code fixes; optionally apply and commit, or loop (refine) until review is clean.
- **Entry points:** New commands `stet fix` and `stet refine`. Register in [cli/cmd/stet/main.go](cli/cmd/stet/main.go) with `rootCmd.AddCommand(newFixCmd())` and `rootCmd.AddCommand(newRefineCmd())` (around line 170).
- **Inputs:** Session from state dir (active findings = [session.Session](cli/internal/session/session.go) Findings minus DismissedIDs). Optional `--finding-id ID` to limit to one finding. Flags: `--dry-run`, `--apply`, `--commit`, `--max-iterations N` (refine), `--message "..."` (commit message). Repo root from cwd. Config: fix_model, fix_temperature (see Config and env).
- **Outputs:** Without `--apply`/`--commit`: proposed patches (unified diff or code blocks) to stdout. With `--apply`: apply patches to working tree. With `--commit`: after apply, create one commit per fix (or per refine iteration) with attribution line in body: `written by <MODEL> after review from stet`. Session and refs/notes/stet unchanged by fix; stet finish continues to write notes as today.
- **Code to reuse:** [session.Load](cli/internal/session/session.go)(stateDir); [findings.Finding](cli/internal/findings/finding.go) (File, Line, Range, Message, Suggestion, Category). [expand.ExpandHunk](cli/internal/expand/expand.go) for hunk-based expansion; **add** `ExpandAtLocation(repoRoot, filePath, startLine, endLine, maxTokens int) (string, error)` in [cli/internal/expand](cli/internal/expand/expand.go) for finding-based context (enclosing function for Go, else N-line window). [ollama.Client](cli/internal/ollama/client.go) `Generate` with fix model. Token estimation in [cli/internal/tokens](cli/internal/tokens) or equivalent. Resolve finding by prefix: [findings.ResolveFindingIDByPrefix](cli/internal/findings/finding.go).
- **Implementation chunks:**
  1. Add `expand.ExpandAtLocation(repoRoot, filePath, startLine, endLine, maxTokens)` with same semantics as roadmap table (enclosing function for Go; ±50/30/20/10 lines fallback; minimal ±2–5). Unit tests with fixture files.
  2. Fix pipeline (new package or under run): load session, filter to active findings or single finding by ID; for each finding compute token budget, extract context (ExpandAtLocation or N-line window), build fix prompt (system: "You are a code fix assistant…"; user: file, line/range, message, suggestion, category, code context); call Ollama with fix model; parse response (code block or unified diff). Unit tests with mock Ollama.
  3. `stet fix` CLI: flags, call fix pipeline, print patches; with `--apply` apply to worktree; with `--commit` run git commit with attribution line. Integration tests (e.g. dry-run, apply in temp repo).
  4. `stet refine` CLI: loop — fix → apply & commit → run review (invoke run package); repeat until no active findings or `--max-iterations`. Default `--max-iterations 3`. Tests with dry-run and mock.
  5. Config and env: add fix_model (default `qwen2.5-coder:7b`), fix_temperature (default 0.1) to [config.Config](cli/internal/config/config.go); env `STET_FIX_MODEL`, `STET_FIX_TEMPERATURE`.
  6. Optional: when git-ai CLI is available and repo uses it, record fix in `refs/notes/ai` per [Git AI Standard v3.0.0](https://github.com/git-ai-project/git-ai/blob/main/specs/git_ai_standard_v3.0.0.md) (agent tool=stet, model=fix_model, session id e.g. stet-{session_id}-{finding_short_id}).
- **Context extraction (reference):** Largest-first, fallback-to-smaller. Chunk levels: Enclosing function (Go via expand); ±50 lines; ±30/20/10; minimal ±2–5. Algorithm: token budget = contextLimit - systemPrompt - findingPayload - responseReserve; try largest chunk; if over budget, fall back to smaller N-line windows; last resort minimal range.
- **Commit message and attribution:** Commit body must include line: `written by <MODEL> after review from stet`. User may add subject/body via `--message`. Refine: one commit per iteration; cap with `--max-iterations`; "passes" means stet review reports no active findings (stet does not run tests/linters/CI).
- **Prompt design (reference):** System: "You are a code fix assistant. Given a code snippet and a code review finding, output ONLY the corrected code. Do not explain." User: file path, line/range, message, suggestion (if any), category, code context. Output format: code block or unified diff for parsing.
- **Config and env:** `fix_model` (default `qwen2.5-coder:7b`), `fix_temperature` (default 0.1). Env: `STET_FIX_MODEL`, `STET_FIX_TEMPERATURE`. Add to [config.Config](cli/internal/config/config.go) and Overrides.
- **Acceptance criteria:** `stet fix` without `--apply`/`--commit` prints patches to stdout. With `--apply` patches are applied to worktree. With `--commit` one commit is created with attribution line. `stet refine` runs fix → commit → review until no active findings or max-iterations; each iteration produces one commit with attribution.
- **Tests:** New and changed code must meet project coverage: 77% project, 72% per file (see [AGENTS.md](../AGENTS.md)).

---

## Phase 9: The "Adaptive" Release (Personalization)

**Goal:** Reduce "False Positive Fatigue" by learning from dismissals and team rules.

### 9.1 Feature: Dynamic Suppression (The "Shut Up" Button)

- **Status:** Not started.
- **Goal:** If the user repeatedly dismisses similar feedback, stet should reduce or stop offering similar findings (via prompt injection or post-process filter).
- **Entry points:** No new command. Integrate into the review path. Call sites: when building the system prompt or when post-processing findings — [cli/internal/review/review.go](cli/internal/review/review.go) and/or [cli/internal/run/run.go](cli/internal/run/run.go). Prefer Option A (prompt injection) first; Option B (post-process similarity) can follow as an optional chunk.
- **Inputs:** Last N dismissed findings from `.review/history.jsonl`. Schema: [history.Record](cli/internal/history/schema.go) with UserAction.Dismissals and ReviewOutput; see [history.ReadRecords](cli/internal/history/append.go) (or equivalent) for reading in chronological order. Config: enable/disable, N (e.g. 50).
- **Outputs:** Fewer findings shown to the user: either the system prompt includes "Do not report issues similar to: [examples]" (Option A) or new findings are filtered by similarity to dismissed (Option B). Config flag to disable suppression.
- **Code to reuse:** [history](cli/internal/history) — ReadRecords / append order; [history.Dismissal](cli/internal/history/schema.go) and Finding message for building examples. [prompt](cli/internal/prompt/prompt.go) — add `AppendSuppressionExamples(systemPrompt string, examples []string) string`. Wire into review.go after SystemPrompt and before or after other append steps.
- **Implementation chunks:**
  1. Read last N history records (e.g. 50) from state dir; extract Dismissals and corresponding finding message (and file:line) for each. Unit test: fixture history.jsonl, assert extracted examples.
  2. Build short "example" text per dismissal (e.g. message + file:line). Deduplicate or limit total length. Add `AppendSuppressionExamples(systemPrompt, examples)` in prompt package; fixed section header e.g. "Do not report issues similar to:".
  3. In review path (review.go and/or run.go), call AppendSuppressionExamples when suppression enabled and examples non-empty. Config flag: e.g. `suppression_enabled` (default true or false), `suppression_history_count` (default 50).
  4. Unit and integration tests: with suppression on and fixture history, assert system prompt contains the section; optionally assert fewer findings in dry-run. Optional later chunk: vector store and post-process similarity filter (similarity > threshold → suppress).
- **Config and env:** e.g. `suppression_enabled` (bool), `suppression_history_count` (int, default 50) in [config.Config](cli/internal/config/config.go); env `STET_SUPPRESSION_ENABLED`, `STET_SUPPRESSION_HISTORY_COUNT`.
- **Acceptance criteria:** After dismissing several findings and running review again, the system prompt includes "Do not report issues similar to" with recent examples (when enabled). With suppression disabled, behavior unchanged. No new CLI commands.
- **Tests:** New and changed code must meet project coverage: 77% project, 72% per file (see [AGENTS.md](../AGENTS.md)).

### 9.2 Feature: Team Rulebook (`.stet/rules.md`)

- **Status:** Not started.
- **Goal:** Allow teams to enforce natural-language rules (e.g. naming, no fmt.Printf in production) by injecting them into the system prompt as high-priority constraints.
- **Entry points:** No new command. Load file when building the system prompt. Call site: same chain as Cursor rules — [cli/internal/review/review.go](cli/internal/review/review.go) loads system prompt then calls [AppendCursorRules](cli/internal/prompt/prompt.go); add a step to load and append rulebook (e.g. before or after Cursor rules). Same integration point in [cli/internal/run/run.go](cli/internal/run/run.go) where system prompt is built.
- **Inputs:** File at repo root `.stet/rules.md` (or configurable path via config). Encoding: UTF-8. If file is missing or unreadable, skip (no error). Reasonable size limit (e.g. 64 KiB) to avoid huge prompts.
- **Outputs:** Rules content appended to the system prompt under a fixed section header (e.g. "## High Priority Constraints"). No change to findings schema.
- **Code to reuse:** [prompt.SystemPrompt](cli/internal/prompt/prompt.go), [prompt.AppendCursorRules](cli/internal/prompt/prompt.go). Review chain in [review.ReviewHunk](cli/internal/review/review.go) (SystemPrompt → InjectUserIntent → AppendCursorRules → …). Add `AppendRulebook(systemPrompt, rulebookPath string) string` in prompt package; if rulebookPath is non-empty and file exists, read and append with header. Repo root from [git.RepoRoot](cli/internal/git).
- **Implementation chunks:**
  1. Resolve path: repo root + `.stet/rules.md` or config `rules_file` if present. If config, add optional `rules_file` to [config.Config](cli/internal/config/config.go).
  2. Read file; validate (exists, size within limit). Return empty string if missing. Add `AppendRulebook(systemPrompt, rulebookPath string) string` in [cli/internal/prompt](cli/internal/prompt/prompt.go).
  3. Wire in review.go and run.go: after SystemPrompt (and optionally after InjectUserIntent), call AppendRulebook with resolved path.
  4. Document format (Markdown) and precedence vs Cursor rules (e.g. rulebook = global team; Cursor rules = file-glob specific).
  5. Unit tests: AppendRulebook with missing file, empty file, and valid content; integration test: run with fixture .stet/rules.md, assert prompt contains section.
- **Config and env:** Optional `rules_file` in config (path relative to repo or absolute); if unset, use repo root `.stet/rules.md`.
- **Acceptance criteria:** When `.stet/rules.md` exists at repo root, the system prompt includes "## High Priority Constraints" and the file contents. When file is missing, no error and no section added.
- **Tests:** New and changed code must meet project coverage: 77% project, 72% per file (see [AGENTS.md](../AGENTS.md)).

### 9.3 Feature: Feedback-based RAG and Strictness Tuning

- **Status:** Not started.
- **Goal:** Use dismissal (and optionally acceptance) history to suggest configuration changes (RAG symbol limits, strictness) that correlate with better acceptance or lower false-positive dismissal; suggest-only, no auto-apply.
- **Entry points:** Extend `stet optimize` or add a new command (e.g. `stet suggest-config`). Current optimizer: [cli/cmd/stet/main.go](cli/cmd/stet/main.go) `runOptimize` invokes an external script that writes `system_prompt_optimized.txt`. Either extend the script contract to also write suggested config, or add a Go path that reads history and writes suggested config snippet (e.g. `.review/suggested_config.toml` or stdout).
- **Inputs:** [.review/history.jsonl](cli/internal/history/schema.go) with [Record](cli/internal/history/schema.go) (ReviewOutput, UserAction.Dismissals, RunConfig). RunConfig (RunConfigSnapshot) already has Strictness, RAGSymbolMaxDefinitions, RAGSymbolMaxTokens. Ensure records are tagged with run config when available (append path may already support RunConfig).
- **Outputs:** Suggested config snippet: e.g. "suggested rag_symbol_max_definitions: 8", "suggested strictness: lenient", or a file `.review/suggested_config.toml` (or printed to stdout). No auto-apply; user merges manually.
- **Code to reuse:** [history.ReadRecords](cli/internal/history/append.go) for chronological read; [Record.RunConfig](cli/internal/history/schema.go), [Record.UserAction.Dismissals](cli/internal/history/schema.go). Aggregate by config buckets; compute dismissal rate (e.g. false_positive / total) and acceptance rate; suggest RAG/strictness values that correlate with higher acceptance or lower FP rate. [config](cli/internal/config/config.go) key names for output.
- **Implementation chunks:**
  1. Confirm history schema and append path populate RunConfig when available; extend if needed. No new CLI flags required for this chunk.
  2. Analyzer: read history, group by RunConfig (or bins), compute per-group dismissal rate and acceptance rate; choose suggested values (e.g. config with best acceptance in last N records). Scope: RAG symbol options and strictness only.
  3. Output: write `.review/suggested_config.toml` or print to stdout (e.g. `stet suggest-config`). Document format.
  4. If extending `stet optimize`: document that script may write both system_prompt_optimized.txt and suggested_config.toml; CLI does not need to read suggested_config unless adding a merge command later.
  5. Unit tests: fixture history with varied RunConfig and dismissals, assert suggested values. Integration test optional.
- **Config and env:** No new config keys; output is suggested config for user to merge.
- **Acceptance criteria:** After enough history with varied run config, running the suggest path produces suggested rag_symbol_max_definitions, rag_symbol_max_tokens, and/or strictness. User can copy into .review/config.toml. No automatic application.
- **Tests:** New and changed code must meet project coverage: 77% project, 72% per file (see [AGENTS.md](../AGENTS.md)).

---

## Phase 10: The "Deep Context" Release (Graph Awareness)

**Goal:** Detect when a change in one file breaks logic in another file that wasn’t touched ("spooky action at a distance").

### 10.1 Feature: Cross-File Impact Analysis

- **Status:** Not started.
- **Goal:** Emit findings when a change in File A (e.g. signature of a function) likely breaks File B that uses that symbol but was not updated (e.g. test file not in diff).
- **Entry points:** No new command. Integrate into the review pipeline as an extra pass or extra findings. Call site: after or alongside per-hunk review in [cli/internal/run/run.go](cli/internal/run/run.go). For changed hunks, extract public symbols, search repo for usages in other files; if a referencing file is not in the diff, emit a finding.
- **Inputs:** Changed hunks from diff (from existing diff pipeline); repo root. Symbol extraction: public symbols (e.g. function names, type names) in changed hunks — use Tree-sitter or extend existing [rag](cli/internal/rag/rag.go) symbol layer. Reference search: grep/ripgrep or LSP to find usages of those symbols in other files.
- **Outputs:** Additional [findings.Finding](cli/internal/findings/finding.go) values (e.g. "You changed Login signature; auth_test.go is stale. This will likely break the build.") added to session or streamed with same schema. No new severity/category required; use existing.
- **Code to reuse:** [diff](cli/internal/diff) for hunks; [rag](cli/internal/rag/rag.go) for symbol definitions (extend for "symbols defined in hunk" and "references in repo" if needed). [findings.Finding](cli/internal/findings/finding.go) for constructing new findings. Optionally Tree-sitter for parsing changed files.
- **Implementation chunks:**
  1. Symbol extraction from changed hunks (per language; start with Go). Produce list of (symbol name, file, line). Unit test: fixture hunk, assert extracted symbols.
  2. Reference search: for each symbol, find references in repo (e.g. grep/ripgrep by symbol name, or LSP). List (file, line) of references; exclude files that are in the diff.
  3. For each reference (file not in diff): generate one finding (e.g. "You changed X; <file> uses X and was not updated. This will likely break the build."). Merge into review output (append to findings in run.go).
  4. Config to enable/disable (e.g. `cross_file_impact_enabled`). Default off for initial rollout.
  5. Unit and integration tests: fixture repo with changed function and untouched caller, assert finding produced when enabled.
- **Config and env:** e.g. `cross_file_impact_enabled` (bool, default false) in [config.Config](cli/internal/config/config.go); env `STET_CROSS_FILE_IMPACT_ENABLED`.
- **Acceptance criteria:** When enabled, changing a function signature (or renamed symbol) in a file and not updating another file that references it produces an actionable finding. When disabled, no cross-file findings.
- **Tests:** New and changed code must meet project coverage: 77% project, 72% per file (see [AGENTS.md](../AGENTS.md)).

---

## Feature: Finding Consolidation (Post-Processing)

- **Status:** Not started.
- **Goal:** Group findings that represent the same underlying issue (same file + similar message or category) so users see conceptual groups instead of many near-duplicate lines. Display-only; session unchanged.
- **Entry points:** `stet list --grouped` and optional `stet list --grouped --verify`. Extend [newListCmd](cli/cmd/stet/main.go) (list command, around line 913); add flags `--grouped` and `--verify`.
- **Inputs:** Active findings from session (same as current `stet list`): load session, compute active = Findings minus DismissedIDs. Use [findings.Finding](cli/internal/findings/finding.go) (File, Line, Range, Message, Category). With `--verify`, optional LLM call for borderline groups (narrow yes/no prompt).
- **Outputs:** Display-only; session and findings unchanged. When `--grouped`: print grouped format — for each group a line `[Group: <canonical description>]` then member lines (shortID, file:line, severity, message). Same info as list, reordered and grouped. With `--verify`, optionally merge groups that LLM says are same issue.
- **Code to reuse:** [hunkid.messageStem](cli/internal/hunkid/hunkid.go), [hunkid.collapseWhitespace](cli/internal/hunkid/hunkid.go) for message normalization. [findings.ShortID](cli/internal/findings/finding.go). New package e.g. `cli/internal/consolidate` for grouping logic (pure functions: GroupFindings(findings) → groups).
- **Implementation chunks:**
  1. Grouping logic in `cli/internal/consolidate`: (a) Same file + message stem similarity (use messageStem/collapseWhitespace; Jaccard on word stems or Levenshtein below threshold). (b) Same file + shared category + overlapping keywords. (c) Same file + nearby lines (e.g. within 20) + similar message stem. Return list of groups, each with canonical description (e.g. first finding’s message or merged) and member findings. Unit tests with fixture findings.
  2. `stet list --grouped`: load active findings, call GroupFindings, print format below. No `--verify` yet. Integration test: session with multiple similar findings, assert output format.
  3. Optional `--verify`: for borderline groups (e.g. similarity in range 0.6–0.85), call small LLM with prompt "Are these N findings the same underlying issue? Answer Yes or No." Merge groups when Yes. Gate behind `--verify`; default off.
  4. Document output format and thresholds (config optional: e.g. similarity threshold).
- **Output format (reference):**

```text
[Group: Potential nil pointer dereference in StreamOut]
  ca1b234  cli/cmd/stet/main.go:301  warning  Potential nil pointer dereference in StreamOut assignment
  3a0ed5c  cli/cmd/stet/main.go:498  warning  Potential nil pointer dereference in StreamOut assignment

[Group: Duplicate function definition for newRunCmd]
  f0b4f0c  cli/cmd/stet/main.go:356  error  Duplicate function definition for newRunCmd
  54e82de  cli/cmd/stet/main.go:363  error  Duplicate function definition for newRunCmd
```

- **Config and env:** Optional similarity threshold in config; `--verify` is opt-in flag. No required config for minimal version.
- **Acceptance criteria:** `stet list --grouped` prints findings grouped by same file + similar message/category; session and dismiss behavior unchanged. `stet list` without `--grouped` unchanged. With `--verify`, borderline groups may be merged after LLM confirmation.
- **Tests:** New and changed code must meet project coverage: 77% project, 72% per file (see [AGENTS.md](../AGENTS.md)).

---

## Research and spikes

| Topic | Goal | Complexity |
|-------|------|------------|
| Local vector stores | Evaluate sqlite-vss vs chromadb for storing dismissal history locally without heavy dependencies. | Medium |
| LSP integration | Use running Language Server (LSP) instead of or in addition to Tree-sitter where possible. | High |
| Review summarization | Generate a "PR Description" from findings (auto-draft PR). | Low |
| Documentation quality | Use commit (and future PR) description for intent context; document that clear author-side docs improve stet accuracy. | Low |
| Evaluation corpus | Fixed set of hunks (known-good / known-bad) to track precision/recall as prompts and optimizer change. | Medium |

**Adoption:** Pilot on one team or repo; collect dismiss reasons and run `stet optimize` periodically; document when to use default vs. strict vs. nitpicky so rollout aligns with feedback-driven improvement.
