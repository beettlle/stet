# Unified Report: Mistral Vibe → Stet — Concepts and Recommendations

**Purpose:** Single reference merging all five analyses (original Vibe deep-dive + Reports 1–4) into one set of actionable recommendations for improving stet, especially for 32k vs 256k context usage and local-first review.

---

## 1. Executive Summary

Mistral Vibe is a CLI coding agent (256k context, local/cloud, tools + MCP). Stet is review-only, Ollama-backed, with a 32k default and many users on 256k. Five reports were synthesized to identify concepts stet can adopt without changing its review-only design.

**Top priorities for "half 32k, half 256k" and large-context behavior:**

- **Structured project and change-scope context** — Optional session-level repo snapshot and/or change-scope block (files, line counts, shallow tree) so the model has a consistent map; bounded by config (e.g. `project_context_max_chars`, 128k threshold).
- **Explicit context budgeting and middleware-style handling** — One documented context-budget schema, config knobs, per-hunk warnings when near limit, optional single-prompt compaction (trim/summarize heaviest segments), and suppression summarization instead of blind truncation.
- **Named review modes/profiles** — Bundled presets (e.g. full, quick, strict, plan, edits-only) that set strictness, RAG, suppression, nitpicky, and post-filters so 32k vs 256k users can pick a mode without juggling flags.

**Additional high-value areas:** Trust/safety checks (dangerous-directory, optional "trust this repo?", confirm before writing in new paths); session resumption and observability (session IDs, persisted token usage, `--continue`/`--resume`); AGENTS.md injection; review skills (prompt + category/focus + optional RAG/post-processors); programmatic limits (`--max-hunks`, `--max-duration`); and optional provider abstraction for 256k backends (vLLM, LiteLLM).

---

## 2. Recommendations by Theme

### 2.1 Project and Change-Scope Context (High Impact)

**Vibe:** ProjectContextProvider builds a single system-prompt block at session start: directory tree (depth/file/char limited, .gitignore-aware), git status, recent commits, and when truncated a "use tools to explore" note. Config: max_chars, max_depth, max_files, timeout_seconds.

**Stet today:** Per-hunk context only (diff + expand + RAG + rules). No shared repo-level or change-level map.

**Unified recommendation:**

- **Change-scope block (diff-focused):** Add an optional "Change scope" section to the system prompt: list of changed files, line counts per file, optional shallow tree of changed paths. Small (e.g. < 500 tokens). Gives the model a map of the diff before each hunk.
- **Bounded project-context block (repo-focused):** When `context_limit >= 128k` (or config), add an optional **session-level** block built **once per stet start / stet run** so every hunk sees the same high-level repo picture: limited tree + branch + git status + last N commits. When stet detects it is running with effective context ≥ 128k (from config or model-reported context length), enable the project-context block by default; allow config to override (e.g. disable or reduce size). Apply explicit caps (e.g. max_chars, max_depth, max_files, timeout) and a fallback line: "This is a summary; rely on the diff and symbol definitions for details." Config: e.g. `project_context_max_chars` so 32k users can disable or keep it very small.
- **Single source of truth:** Both blocks are size-capped and configurable; 32k users keep current behavior by disabling or minimizing project context.

### 2.2 Context Budget and Middleware-Style Management (High Impact for 256k)

**Vibe:** Middleware pipeline before each turn: ContextWarningMiddleware (warn at e.g. 50% of context), AutoCompactMiddleware (at threshold, summarize conversation and replace history with system + summary). Explicit ProjectContextConfig and auto_compact_threshold in config.

**Stet today:** `WarnIfOver` once before the pipeline; scattered caps (context_limit, rag_symbol_*, maxRAGTokensWhenContextLarge, maxSuppressionTokensCap, etc.). No per-hunk warning or compaction.

**Unified recommendation:**

- **Context-budget schema:** Define one documented schema (in config or docs) for segment limits: system prompt, project summary, per-hunk expand, RAG, suppression, response reserve, and any future project-context block, each with explicit max tokens/chars and **precedence** when over budget. Unifies existing caps and makes 32k vs 256k behavior clear.
- **Config knobs:** Expose options such as `project_context_max_chars`, `context_warn_at_ratio` (e.g. warn when a hunk's prompt exceeds 90% of context_limit) so 32k/256k can tune without code changes.
- **Priority when over budget:** Apply a clear order: keep system + hunk first, then RAG, then suppression (trim or summarize).
- **Summarize suppression:** Instead of dropping the last N prompt shadows, inject a short summary: "User has dismissed N similar findings; patterns include: X, Y, Z."
- **Per-hunk warning:** When a hunk's estimated prompt exceeds a configured fraction of context_limit, emit a one-line stderr warning and/or an optional line in **that hunk's** system prompt: "This prompt is near the context limit; focus on high-signal findings."
- **Single-prompt compaction:** For near-limit hunks, trim or summarize the heaviest segments (e.g. more aggressive RAG cap, or a one-line "compact" representation of the system prompt) so the model still gets essential instructions without hitting the limit.
- **Budget line in system prompt (optional):** For large context_limit, add a short line: "Remaining context budget: ~X tokens. Prioritize concise, high-signal content."
- **Observability:** Persist or report per-run/per-session prompt and completion tokens and duration (build on existing HunkUsage/stream usage) to support context-warning logic and cost/quality tuning.
- **Auto-compact for conversation:** Reserve full conversation-style compaction (summarize then reset to system + summary) for **future** multi-turn flows (e.g. "explain this finding," "suggest a fix"); not required for current single-pass per-hunk pipeline.

### 2.3 Review Modes and Profiles (Medium Impact)

**Vibe:** Multiple agent profiles (default, plan, accept-edits, auto-approve) with different tool permissions and system prompts; `--agent` to choose.

**Stet today:** Single review path with strictness, nitpicky, and optional critic as separate knobs.

**Strictness vs. modes:**

- **Strictness today (single dimension):** One setting that controls (1) abstention thresholds (e.g. strict 0.6/0.7, default 0.8/0.9, lenient 0.9/0.95) and (2) whether the FP kill list is applied (the `+` presets skip it). Nitpicky is separate: it adds convention/typo instructions to the prompt and disables the FP kill list. So today: `--strictness` plus `--nitpicky`; RAG/suppression/context are configured independently. Strictness does **not** change RAG size, prompt text, or output shape (e.g. "findings only" vs "suggestions").
- **Review modes/profiles (proposed):** Named presets that **bundle** strictness, nitpicky, RAG/suppression caps, and optionally prompt or output shape (e.g. plan = findings only, minimal suggestion text; edits-only = only findings that imply concrete edits). One choice sets several knobs so users can pick "quick" vs "deep" vs "plan" without juggling flags. Modes are a convenience layer: they can map to existing strictness/nitpicky and add RAG/suppression defaults and, for some modes, different instructions or output forms.

| | Strictness (today) | Review mode/profile (proposed) |
| -- | -- | -- |
| **Scope** | Filter behavior only (thresholds + FP kill list) | Kind of review = strictness + nitpicky + RAG + suppression + optional prompt/output |
| **UX** | "How picky is the filter?" | "What kind of review?" — one choice sets many knobs |

**Unified recommendation:**

- **Named modes/profiles:** Introduce `--mode` (or config) with bundled behavior, e.g.:
  - **full** / **review** — Current full review (default).
  - **quick** — Fewer RAG/suppression tokens, faster runs.
  - **strict** — Stricter confidence thresholds, current "strict" preset.
  - **plan** — Findings only, minimal or no suggestion text (lighter prompts for 32k).
  - **edits-only** — Only findings that imply concrete code edits; downplay style/convention unless nitpicky.
  - **security** (optional) — Stricter security checks, higher confidence bar.
- Each mode sets strictness, RAG, suppression, nitpicky, and optionally which post-filters run. This gives a Vibe-like "pick a mode" experience and makes 32k vs 256k usage easier (e.g. "quick" vs "deep").

### 2.4 Trust and Safe-Directory (Medium Impact)

**Vibe:** Trusted folders in `~/.vibe/trusted_folders.toml`; when cwd has `.vibe` and is not yet trusted, prompt the user; dangerous-directory check (e.g. home, system dirs) replaces project context with a "do not run here" message.

**Stet today:** No explicit trust or dangerous-directory check.

**Unified recommendation:**

- **stet doctor or startup check:** Warn or refuse when repo root (or cwd) is under `$HOME`, `/`, `/etc`, `/usr`, or a **configurable blocklist**.
- **Optional first-time "trust this repo?":** When running `stet start` in a repo for the first time (or when `.review` is present in a previously unseen path), optionally prompt and require confirmation before writing; record in `~/.config/stet/trusted_repos` or `.review`.
- **Confirm before writing:** When `.review` exists in a new path, require explicit confirmation before creating/updating session state.
- Document "trusted repos" and blocklist so teams can avoid accidental runs on sensitive trees.

### 2.5 Session Resumption and Observability (Medium Impact)

**Vibe:** Session logging; `--continue` / `--resume SESSION_ID` to continue from last or a specific session.

**Stet today:** Session state (baseline, last_reviewed_at, findings, dismissed) and incremental `stet run` (only new/changed hunks). No session IDs or resume across invocations.

**Unified recommendation:**

- **Session IDs and optional session logging:** Persist which hunks were reviewed and token usage per run/session.
- **Resume/continue:** Support `stet run --continue` (last session) or `stet run --resume <id>` so users and scripts can "resume where I left off." Design for future multi-turn use; lower priority for current single-shot flow.
- **Incremental persistence:** During long runs, persist session state incrementally so an interrupted `stet start` can be resumed from last_reviewed_at.
- Use session data later for context-warning or compaction in long-running workflows.

### 2.6 Extensibility: AGENTS.md and Review Skills (Medium Impact)

**Vibe:** Loads AGENTS.md from project root when trusted; skills from `.agents/skills/`, `.vibe/skills/`, `~/.vibe/skills/` with SKILL.md (name, description, allowed-tools, user-invocable). Skills can add tools and slash commands.

**Stet today:** `.cursor/rules/` and optional `system_prompt_optimized.txt`; no first-class "review skill" or discovery from multiple dirs.

**Unified recommendation:**

- **AGENTS.md:** When present at repo root, load and inject as an always-apply "Project context" (or "Project overview") block with a token/char cap. Low effort, high signal; many projects already use AGENTS.md.
- **Review skills:** Discover skills from `.review/skills/` (and optionally `.agents/skills/` or a configurable/global path). Each skill = prompt fragment + optional **allowed categories or focus** (e.g. security-only, perf-only) + optional **extra RAG sources** or **post-processors** (e.g. security skill with a second pass). Config: `enabled_skills = ["security", "perf"]`. Complements CursorRules; allows focused reviews and smaller prompts for 32k and multiple skills for 256k.

### 2.7 Programmatic and Operational Limits (Medium Impact)

**Vibe:** Programmatic mode: `--max-turns`, `--max-price`, `--enabled-tools`, `--output json|streaming`.

**Stet today:** No max-hunks or max-duration; streaming (NDJSON) exists for extension.

**Unified recommendation:**

- **Flags:** `--max-hunks N` (stop after N hunks, useful for CI or quick checks), `--max-duration D` (stop after D seconds). Optional `--output streaming` (NDJSON) where not already the default for the extension.
- Supports partial reviews, CI use, and cost control for local inference.

### 2.8 Provider and Model Abstraction (Lower Priority, Enables 256k Backends)

**Vibe:** ProviderConfig + ModelConfig; Mistral API and llamacpp (Ollama-compatible); model aliases; Devstral Small 2 for local (~32GB RAM).

**Stet today:** Ollama-only; single model; `ollama_base_url` and model name.

**Unified recommendation:**

- **Provider abstraction:** Keep Ollama as default; add support for generic OpenAI-compatible endpoints (vLLM, LiteLLM, llama.cpp). Optional model aliases in config (e.g. `qwen3-coder:30b` → `local-review`) and per-model defaults for temperature and num_ctx. Helps 256k-capable backends without changing core review behavior.

### 2.9 Subagents / Task Delegation (Optional, Future)

**Vibe:** `task` tool delegates to subagents (e.g. "explore") so the main agent doesn't hold all exploration context.

**Stet today:** Single pipeline; no delegation.

**Unified recommendation:**

- **Principle only:** If stet ever adds a "scope" or "plan" pass (e.g. a separate read-only call that returns hunk ordering or a PR summary), keep that context out of the main per-hunk loop. Optional; not required for current design.

---

## 3. Priority and Effort Summary

| Theme | Recommendation | Effort | Impact (32k / 256k) |
| ------------------------------ | ------------------------------------------------------------------------------------------------------------------------------ | ---------- | ------------------------------------- |
| Project / change-scope context | Change-scope block + optional bounded project-context block (session-level, 128k threshold, config caps) | Medium | High for 256k; optional for 32k |
| Context budget and middleware | Budget schema, config knobs, priority order, suppression summarization, per-hunk warning, single-prompt compact, observability | Medium | High for 256k; clarity for both |
| Review modes/profiles | `--mode` full / quick / strict / plan / edits-only (and optional security) | Low–Medium | High usability; 32k/256k both |
| Trust / safe-directory | stet doctor + blocklist + optional "trust this repo?" + confirm before write | Medium | Safety; both |
| Session resumption | Session IDs, logging, `--continue` / `--resume`, incremental persistence | Medium | Usability; scripts; future multi-turn |
| AGENTS.md | Load and inject when present, token-capped | Low | High value, low effort |
| Review skills | `.review/skills/` discovery; prompt + category/focus + optional RAG/post-processors | Medium | Extensibility; focused prompts |
| Programmatic limits | `--max-hunks`, `--max-duration`, streaming | Low | CI; partial reviews |
| Provider abstraction | Ollama + OpenAI-compatible backends; aliases; per-model defaults | Medium | 256k backends (vLLM, etc.) |
| Subagents / delegation | Optional scope/plan pass in future | Future | Future multi-step flows |

**Suggested implementation order (first phase):**

1. **Context-budget schema and config knobs** — Document and expose `project_context_max_chars`, `context_warn_at_ratio` (and any existing caps) so behavior is predictable.
2. **Change-scope block** — Add optional "Change scope" section (files, line counts, shallow tree) to system prompt; keep it small and configurable.
3. **Review modes** — Introduce `--mode` with full / quick / strict (and later plan / edits-only) mapping to strictness, RAG, suppression, nitpicky.
4. **AGENTS.md injection** — Load from repo root when present; inject as always-apply project context with cap.
5. **Per-hunk context warning** — When estimated prompt exceeds configured fraction of context_limit, warn on stderr and optionally add one line to that hunk's system prompt.
6. **Suppression summarization** — When prompt shadows are trimmed, replace with "User has dismissed N similar; patterns: X, Y, Z."
7. **Optional project-context block** — When effective context ≥ 128k (detected from config or model), enable project-context block by default; build once per run; apply caps and "summary; rely on diff/symbols" note; allow config to disable or limit.
8. **Trust/safety** — stet doctor or startup check for dangerous cwd; optional blocklist and "trust this repo?" flow.
9. **Session IDs and resume** — Persist session ID and token usage; add `--continue` / `--resume`.
10. **Programmatic limits** — `--max-hunks`, `--max-duration`.

---

## 4. Tradeoffs and Design Notes

- **Project context vs. per-hunk size:** Project and change-scope blocks use tokens once per run; for 32k they should be off or very small so per-hunk RAG/expand still fit. The 128k threshold (or config) keeps 32k default behavior unchanged.
- **Compaction:** Single-prompt compaction (trim/summarize heaviest segments) applies to today's per-hunk flow; full conversation compact is only for future multi-turn. Avoid overloading the current pipeline with conversation-style compaction.
- **Skills vs. CursorRules:** Skills complement `.cursor/rules/`—project- or team-specific review extensions (categories, focus, optional RAG/post-processors) without replacing CursorRules.
- **Provider abstraction:** Stet stays review-only; adding OpenAI-compatible backends is for flexibility (e.g. 256k on vLLM) without changing the review contract.
- **Session resume:** Current value is incremental persistence and resume after interrupt; full value increases if stet later adds multi-turn or conversational review.

---

## 5. References

- **Original Vibe deep-dive:** [github.com/mistralai/mistral-vibe](https://github.com/mistralai/mistral-vibe); `vibe/core/system_prompt.py`, `vibe/core/middleware.py`, `vibe/core/llm/`, config and prompts.
- **Stet:** [PRD](PRD.md), [review-process-internals](review-process-internals.md), [context-enrichment-research](context-enrichment-research.md).
- **Reports merged:** Original (5th) + Report 1 (change-scope, profiles, middleware, skills, AGENTS.md, session resumption, programmatic limits) + Report 2 (repo snapshot 128k, auto-compact, budget line, trust, modes, skills, provider) + Report 3 (bounded caps, single-prompt compact, budget schema, plan/edits-only, trust first-time, skills category/focus, session for multi-turn) + Report 4 (session-level context once per run, config knobs, observability, per-hunk note, trust blocklist+confirm, skills RAG/post-processors, session data for middleware).
