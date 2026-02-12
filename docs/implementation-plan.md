# Stet Implementation Plan

This document is the implementation roadmap for Stet. It turns the [Product Requirements Document](PRD.md) into phased, testable work: each phase delivers a self-contained artifact with tests and coverage gates. The goal is to reach a state where the product can be used to code-review itself (dogfood) as soon as possible, then complete the CLI and Cursor extension.

**Source of truth:** Implementation follows [docs/PRD.md](PRD.md). This plan adds only phasing, coverage rules, and the decisions below.

---

## 1. Decisions and constraints

These are the source of truth for a coding LLM implementing the project.

| Decision | Choice |
|----------|--------|
| **State storage (v1)** | In-repo `.review/` (session, config, lock). `.review/` is in `.gitignore` by default so state does not pollute version control; it can be removed from `.gitignore` later if the team wants to commit state. An override (e.g. `state_dir` config or env) MAY point to an out-of-repo path if needed. |
| **Coverage** | **77%** line coverage for the **whole project**. **No single file** below **72%** line coverage. Apply from Phase 0 onward. CI MUST enforce both (fail if project &lt; 77% or any file &lt; 72%). |
| **Go** | Latest stable Go (e.g. 1.22+). CI uses stable. |
| **Extension** | Cursor-only for now; manifest/engines reflect Cursor. |
| **RAG-lite and prompt shadowing** | Deferred to **Phase 6** so Phases 3–5 focus on the core review path and extension. Testing the product (dogfood) starts at Phase 3. |
| **Dev container** | Development container (Containers.dev) is implemented first (Phase 0.0); contributors and CI use it for a consistent environment. |
| **DSPy optimizer** | Optional. History in `.review/history.jsonl`; on-demand via `stet optimize` (Python sidecar). Output: `.review/system_prompt_optimized.txt`. CLI loads optimized prompt when present; core remains a static Go binary. Optimizer is optional; dev container and CI do not require Python/DSPy for main CLI build and tests. |
| **History / org sync (future)** | `.review/history.jsonl` schema and format MUST be designed so records can be exported or uploaded in bulk in a future phase (e.g. for org-wide learning). Each line is a self-contained JSON object; avoid implicit local-only identifiers that would break when aggregating from many machines. Document the schema; do not hardcode assumptions that history is only ever consumed locally. Real-time or shared DB (e.g. PostgreSQL) is out of scope for v1; periodic upload/export is the intended future direction. |
| **Session end (v1)** | Explicit `stet finish` and extension "Finish review" button. PRD §9 documents the alternative (auto-finish when 0 findings); implementation follows PRD when/if that is adopted. |

---

## 2. Phase overview

| Phase | Focus |
|-------|--------|
| **Phase 0** | Bootstrap and contract: dev container first, then repo layout, findings + config schema; no review logic. |
| **Phase 1** | CLI state and worktree: session, worktree create/remove, `stet start` / `stet finish` skeleton, `stet doctor` stub. |
| **Phase 2** | Diff and hunk identity: diff pipeline, dual-pass hashing, "already reviewed" set; no LLM. |
| **Phase 3** | Ollama and first full review: client, token estimation, prompt + structured output, wire `stet start` / `stet run`, dry-run. **First dogfood milestone.** |
| **Phase 4** | CLI completeness: status, approve, edge cases, git note on finish. |
| **Phase 5** | Extension: spawn CLI, panel, jump to file:line, copy-for-chat, Finish button. |
| **Phase 6** | Optional and polish: CursorRules injection, streaming NDJSON, RAG-lite, prompt shadowing, **DSPy optimizer (`stet optimize`)**, docs, cleanup. |

```mermaid
flowchart LR
  P0[Phase_0_Bootstrap] --> P1[Phase_1_State_Worktree]
  P1 --> P2[Phase_2_Diff_HunkID]
  P2 --> P3[Phase_3_Ollama_Dogfood]
  P3 --> P4[Phase_4_CLI_Complete]
  P4 --> P5[Phase_5_Extension]
  P5 --> P6[Phase_6_Optional]
```

---

## 3. Per-phase detail

### Phase 0: Bootstrap and contract

| Sub-phase | Deliverable | Tests / coverage |
|-----------|-------------|------------------|
| **0.0** | Dev container: `.devcontainer/devcontainer.json` (and optional Dockerfile or dev container features) with Go (latest stable), Node/TypeScript for the extension, and Git. Verify that build and tests run inside the container. Optional: include Python 3 and DSPy (or a `pip install -r optimizer-requirements.txt`) so `stet optimize` can be run in the container. Optimizer is optional; dev container and CI do not require Python/DSPy for main CLI build and tests. | Build and test commands succeed inside the dev container; no coverage target for this sub-phase. |
| **0.1** | Monorepo layout: `cli/` (Go), `extension/` (TypeScript), `docs/`. Root `go.mod`, extension `package.json`. README with build/run. | Builds succeed. No coverage target for scaffold only. |
| **0.2** | Findings schema as code: single source of truth (e.g. Go structs + JSON tags or JSON Schema). Fields: `id`, `file`, `line`/`range`, `severity`, `category`, `message`, `suggestion`, `cursor_uri`. | Unit tests for serialize/deserialize and required-field validation. 77% project, no file &lt; 72%. |
| **0.3** | Config schema and load order: CLI flags &gt; env &gt; repo config &gt; global config &gt; defaults. Keys: `model`, `ollama_base_url`, `context_limit`, `warn_threshold`, `timeout`, `state_dir`, `worktree_root`. Config SHALL include (in a later phase, e.g. Phase 3) keys for **Ollama model options** (e.g. temperature, model context size) per PRD, with defaults. Repo: `.review/config.toml`; global: XDG `~/.config/stet/config.toml`. | Unit tests for config load and precedence. 77% project, no file &lt; 72%. |

**Phase 0 exit:** Dev container works; build works; findings and config are defined and tested; no review logic yet.

---

### Phase 1: CLI state and worktree

| Sub-phase | Deliverable | Tests / coverage |
|-----------|-------------|------------------|
| **1.1** | State schema and storage: `.review/session.json` with `baseline_ref`, `last_reviewed_at`, `dismissed_ids`, optional `prompt_shadows`. Load/save. Single active session per repo via lock (e.g. under `.review/`). | Unit tests: load/save, lock acquire/release, invalid JSON handling. 77% project, no file &lt; 72%. |
| **1.2** | Worktree lifecycle: create read-only worktree at given ref (path derived, e.g. by baseline SHA). List/remove worktree. Handle "worktree already exists" and "baseline not ancestor of HEAD" with clear errors. | Integration tests with temp git repo (create worktree, assert path/ref, remove). 77% project, no file &lt; 72%. |
| **1.3** | `stet start` (skeleton): parse ref (default HEAD). Validate clean worktree, baseline is ancestor of HEAD. Create worktree; write session (baseline, no findings yet). `stet finish`: persist state, remove worktree, clear lock. `stet doctor`: stub (exit 0). | Integration tests: start → session + worktree exist; finish → worktree gone, state persisted. 77% project, no file &lt; 72%. |

**Phase 1 exit:** `stet start` and `stet finish` work without calling the model; session and worktree are real.

---

### Phase 2: Diff and hunk identity

| Sub-phase | Deliverable | Tests / coverage |
|-----------|-------------|------------------|
| **2.1** | Diff pipeline: given baseline and HEAD, produce list of hunks (file path, raw content, context). Respect .gitignore; exclude binary/generated files; document exclusions. Edge cases: empty diff (no hunks), merge commits (document behavior). | Unit tests: known diff → expected hunk count/content. Integration test with fixture repo. 77% project, no file &lt; 72%. |
| **2.2** | Dual-pass hunk ID: Strict hash = path + raw content (CRLF→LF). Semantic hash = path + code-only (strip comments, collapse whitespace). Language-aware comment stripping (or regex per language). Stable finding IDs (e.g. hash of file + range + message stem). | Unit tests: same content → same IDs; comment-only change → same semantic ID, different strict ID. 77% project, no file &lt; 72%. |
| **2.3** | "Already reviewed" set: from `baseline` + `last_reviewed_at` ref, compute hunks that existed at `last_reviewed_at` and have not changed (strict or semantic match). Output: hunks to review (sent to LLM) and approved/skipped. | Unit tests: fixture state + refs → correct to-review vs approved sets. 77% project, no file &lt; 72%. |

**Phase 2 exit:** Exact set of hunks to review and to skip is known; still no LLM.

---

### Phase 3: Ollama and first full review (dogfood milestone)

| Sub-phase | Deliverable | Tests / coverage |
|-----------|-------------|------------------|
| **3.1** | Ollama client: health check (server reachable, model present). Optional context/window check. `stet doctor` uses this; suggest `ollama pull <model>` if needed. **Error output:** When any command fails with Ollama unreachable (exit 2), the CLI MUST print the underlying error (e.g. `Details: %v`) to stderr for troubleshooting. **Model settings:** Ollama client and config SHALL support passing model runtime options (at least temperature, num_ctx) to `/api/generate` with defaults (e.g. low temperature, context 32768). Config keys and defaults SHALL be documented; doctor and review path use these options so the model runs with the correct settings for Stet. | Unit tests with mock HTTP; optional integration test (skip if no Ollama). 77% project, no file &lt; 72%. |
| **3.2** | Token estimation: estimate prompt (and optional response) tokens; configurable context limit and warn threshold; warn when over threshold. Simple estimator (e.g. chars/4 or model-specific if available). | Unit tests: sample prompts → estimated tokens; over limit → warning. 77% project, no file &lt; 72%. |
| **3.3** | Prompt and structured output: build review prompt (hunk + optional context). **Prompt source:** If `.review/system_prompt_optimized.txt` exists, use its contents as the system prompt; otherwise use the default. (Optimized file is produced by `stet optimize`; see Phase 6.) Request JSON matching findings schema (severity, category, message, etc.). Parse response; assign tool-generated finding IDs. Retry once on malformed JSON; then fail or best-effort + warning. | Unit tests: canned JSON → parsed findings; malformed → retry then error. Prompt precedence: when optimized file present vs absent, correct prompt is used (unit or integration). 77% project, no file &lt; 72%. |
| **3.4** | RAG-lite: deferred to Phase 6. No implementation here; prompt uses only hunk content for Phase 3. | N/A |
| **3.5** | Wire `stet start` and `stet run`: start = create worktree, diff baseline..HEAD, for each to-review hunk call Ollama, collect findings, write session (findings + last_reviewed_at = HEAD). run = same baseline, incremental (only to-review hunks), update findings and last_reviewed_at. `--dry-run`: skip LLM, inject canned findings for CI. | Integration tests: dry-run → deterministic findings; with real Ollama (optional) → full run. 77% project, no file &lt; 72%. |
| **3.6** | CLI output contract: emit findings as JSON (or NDJSON for streaming). Exit codes: 0 = success, 1 = usage/error, 2 = Ollama unreachable. Document contract for extension. | Tests: run start with dry-run, parse stdout JSON, assert shape. 77% project, no file &lt; 72%. |

**Phase 3 exit:** Full CLI review path works. **Milestone: use the product to review itself** — run `stet start --dry-run` (and ideally `stet start` with Ollama) on the Stet repo and inspect findings. If multiple stet worktrees remain after interrupted runs, run `stet finish` and remove remaining `.review/worktrees/stet-*` with `git worktree remove <path>` (or use Phase 6 `stet cleanup` when implemented).

---

### Phase 4: CLI completeness

| Sub-phase | Deliverable | Tests / coverage |
|-----------|-------------|------------------|
| **4.1** | `stet status`: report baseline, last_reviewed_at, worktree path, finding count, dismissed count. `stet approve <id>`: add finding ID to approved/dismissed so it does not resurface. **`stet optimize`:** Invoke optional Python optimizer (script or container). Reads `.review/history.jsonl`; runs DSPy optimization; writes `.review/system_prompt_optimized.txt`. Document usage (e.g. run weekly or after enough feedback). Exit codes: 0 = success, non-zero = failure (e.g. missing Python/DSPy, invalid history). No change to core Go binary dependencies. | Unit/integration tests for status output and approve persistence. 77% project, no file &lt; 72%. |
| **4.2** | Edge cases: uncommitted changes on start (error or warn + override); baseline not ancestor; empty diff; worktree already exists; concurrent start (lock). Clear user-facing messages per PRD error table. **Recovery hints:** On `stet start` failure for uncommitted changes or worktree already exists, the CLI prints a one-line hint to stderr (e.g. commit/stash or run `stet finish`) before exiting. | Tests for each edge case; assert messages. 77% project, no file &lt; 72%. |
| **4.3** | Git note on finish: on `stet finish`, write Git Note to `refs/notes/stet` at HEAD (session_id, baseline_sha, head_sha, findings_count, dismissals_count, tool_version, finished_at). Document ref and schema. | Integration test: finish → read note, assert schema. 77% project, no file &lt; 72%. |
| **4.4** | Prompt shadowing: deferred to Phase 6. Optionally stub: store finding_id in dismissed list only; no prompt_shadows yet. | N/A or minimal tests for dismiss list. |
| **4.5** | History for optimizer: append to `.review/history.jsonl` on user actions that indicate feedback (e.g. on dismiss and/or on finish with findings). Each line is a JSON object: e.g. `diff_ref` (or hunk IDs), `review_output` (findings), `user_action` (e.g. `dismissed_ids`, `finished_at`). Schema documented in "State storage" / docs. Bounded size or rotation (e.g. last N sessions) to avoid unbounded growth. Schema and format must be suitable for future periodic export/upload (e.g. for org-wide aggregation) so that a later phase can add upload without breaking changes. | Unit/integration tests: append produces valid JSONL; schema matches doc. 77% project, no file &lt; 72%. |

**Phase 4 exit:** CLI feature-complete per PRD; edge cases and git note done.

---

### Phase 5: Extension (Cursor)

| Sub-phase | Deliverable | Tests / coverage |
|-----------|-------------|------------------|
| **5.1** | Extension scaffold: Cursor extension that spawns CLI; parse JSON (or NDJSON) from stdout; surface errors from stderr/exit code. | Extension loads; unit tests for output parsing. 77% project, no file &lt; 72%. |
| **5.2** | Findings panel: list findings (file, line, severity, category, message). Progress (e.g. "Scanning …"). Click finding → open file:line (file:// / cursor://). | Cursor test runner: panel shows findings; open file at line. 77% project, no file &lt; 72%. |
| **5.3** | Copy for chat: per finding, "Copy for Chat" block — `[File:Line](file://...#L10)`, severity, message (PRD §3e). | Manual or automated: copy produces correct markdown. |
| **5.4** | "Finish review" button: calls `stet finish`; refresh/clear panel. Handle errors (e.g. "Finish or cleanup current review first"). If PRD later adopts "review done when 0 findings," add auto-persist/cleanup on 0 findings and optionally retain explicit Finish for "close session anyway." | Test: finish invoked; panel state updated. 77% project, no file &lt; 72%. |

**Phase 5 exit:** User can run review from IDE, see findings, jump, copy to chat, and finish.

---

### Phase 6: Optional and polish

| Sub-phase | Deliverable | Tests / coverage |
|-----------|-------------|------------------|
| **6.1** | CursorRules / AGENTS.md: if `.cursor/rules/` or `AGENTS.md` exists, discover and inject bounded "Review criteria" into prompt. Optional; tool works without them. | Unit tests: with/without rules files → prompt contains or omits section. 77% project, no file &lt; 72%. |
| **6.2** | Streaming NDJSON: CLI emits progress and findings as NDJSON when requested (e.g. `--stream`). Extension consumes and updates panel incrementally. | Tests: parse NDJSON stream; extension shows incremental updates. 77% project, no file &lt; 72%. |
| **6.3** | RAG-lite: symbol lookup (grep/ctags-style) for symbols used in hunk; inject N definitions (signature + docstring) into prompt; bounded N. | Unit tests: mock file tree + symbol → injected snippet. 77% project, no file &lt; 72%. |
| **6.4** | Prompt shadowing: on dismiss, store (finding_id, prompt_context) in session; optional injection as negative few-shot in future prompts. Prompt shadowing and the optimizer both consume user feedback; optimizer output (`system_prompt_optimized.txt`) takes precedence at prompt build time as stated in 3.3. | Unit tests: dismiss → state updated; optional prompt build test. 77% project, no file &lt; 72%. |
| **6.5** | Docs and cleanup: state storage, config schema, exit codes, extension–CLI protocol, git note ref/schema. Optional `stet cleanup` for orphan worktrees. Document the **optimizer**: when to run `stet optimize`, input (`history.jsonl`), output (`system_prompt_optimized.txt`), and that the core binary does not depend on Python. | No coverage target; docs and behavior verified. |
| **6.6** | DSPy optimizer (optional): Python script (or container) invoked by `stet optimize`. Load `.review/history.jsonl`; define metric (e.g. maximize acceptance, minimize dismissal); use DSPy to compile updated system prompt; write `.review/system_prompt_optimized.txt`. Document: how to run, required Python/DSPy env, and that the Go CLI has no Python dependency. | Optional integration test (skip if no Python/DSPy); unit test for CLI: `stet optimize` exit code when optimizer script missing or failing. 77% project, no file &lt; 72%. |

**Phase 6 exit:** Optional features in place; docs aligned with PRD.

---

## 4. Coverage and quality rules

- **77%** = line coverage for the **entire project**.
- **72%** = minimum line coverage for **every file** (no file below 72%).
- A phase is not complete until: new/changed code has tests, project coverage ≥ 77%, every file ≥ 72%, and existing tests still pass.
- Use fixture repos (e.g. under `testdata/` or `fixtures/`) for diff/worktree/session so tests are deterministic.
- Keep `--dry-run` working after Phase 3 for CI without Ollama.

---

## 5. Reference

Implementation follows [docs/PRD.md](PRD.md). This plan adds phasing, coverage rules, and the decisions in §1.
