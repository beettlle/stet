# Concepts from Dirac for Improving Stet's Token Efficiency and Accuracy

> **Status:** Research complete. Concepts for evaluation and potential adoption.
>
> **Source:** [Dirac](https://github.com/dirac-run/dirac) — an open-source coding agent (Apache 2.0, fork of Cline) that achieves 64.8% lower API costs and 100% accuracy (8/8) on Terminal-Bench-2 complex refactoring tasks using `gemini-3-flash-preview`.
>
> **Why Dirac is relevant:** Stet targets local models with 8k–32k context windows. Dirac's core thesis — that model reasoning degrades as context length grows, so tightly curated context improves both accuracy and cost — aligns directly with stet's constraints. While Dirac is a coding agent (edits code) and stet is a review tool (emits findings), the underlying token-efficiency and context-curation techniques transfer.

---

## Concept 1: File Skeleton as Lightweight Structural Context

### What Dirac does

Dirac provides a `get_file_skeleton` tool that uses Tree-sitter AST parsing to return only the structural outline of a file — function/method/class/interface definition lines — stripping all implementation bodies. This gives the LLM a "map" of the file at minimal token cost (typically 5–15% of full file size). Optionally, the skeleton includes per-function line counts and a call graph (which local functions call which).

### How this applies to stet

Stet currently sends the LLM: (1) the diff hunk, (2) the enclosing function via `expand.ExpandHunk` (Go only), and (3) RAG-resolved external symbol definitions. What's missing is the **local file structure** — the model doesn't know what other functions exist in the same file, how the changed function relates to sibling functions, or where the hunk sits in the file's architecture.

### Proposed adaptation

Add an optional **file skeleton injection** step to `PrepareHunkPrompt`. For the file containing the hunk:

1. Parse the file to extract function/method/type definition signatures (one line each).
2. Mark which function contains the current hunk (e.g. `→ func ProcessReview(...)`).
3. Inject the skeleton into the user prompt under a `## File structure` header.

**Token cost:** A 500-line Go file with 15 functions would produce ~15 signature lines ≈ 60–100 tokens. This is far cheaper than full-file context and provides structural awareness that reduces false positives like "this function is undefined" (the model can see it exists in the skeleton).

**Implementation path:**
- For Go: use `go/ast` (already available, no CGO) to extract `FuncDecl` and `GenDecl` names + signatures.
- For other languages: use Tree-sitter Go bindings (see Concept 2) or a simpler regex-based extractor as a fallback.
- Gate behind a config flag (e.g. `file_skeleton_enabled`, default true) with a token cap (e.g. `file_skeleton_max_tokens`, default 512).

**Relationship to existing work:**
- Complements `expand.ExpandHunk` (enclosing function body) — skeleton gives breadth, expansion gives depth.
- Complements RAG (external symbol definitions) — skeleton gives local file context, RAG gives cross-file context.
- Aligns with `docs/context-enrichment-research.md` Tier 2 (structural context) but goes beyond "parent function" to "sibling functions."

**Where to integrate:** `cli/internal/review/review.go` in `PrepareHunkPrompt`, after hunk expansion and before RAG, as a new `prompt.AppendFileSkeleton(userPrompt, skeleton, maxTokens)` call. New package or function in `cli/internal/expand/` or a new `cli/internal/skeleton/` package.

---

## Concept 2: Tree-sitter as Universal AST Infrastructure

### What Dirac does

Dirac uses Tree-sitter for all AST operations across all supported languages: skeleton extraction, function extraction by name, symbol range finding, and structural editing. One infrastructure layer serves every language-specific need.

### How this applies to stet

Stet currently uses:
- `go/ast` for Go-specific hunk expansion (`expand.ExpandHunk`).
- Regex-based symbol extraction for RAG across languages (Go, JS/TS, Python, Swift, Java, Rust).
- Per-line whitespace minification for Go and Rust only.

The roadmap (`docs/roadmap.md`, "Research and spikes") lists **"AST-preserving minification (non-Go)"** as medium–high complexity. Each language-specific RAG resolver (`cli/internal/rag/go/`, `rag/js/`, `rag/python/`, etc.) implements its own regex-based symbol extraction independently.

### Proposed adaptation

Adopt Tree-sitter (via Go bindings such as `github.com/smacker/go-tree-sitter`) as a shared AST infrastructure layer that consolidates multiple features:

1. **Multi-language minification:** Extend `cli/internal/minify/` beyond Go/Rust whitespace reduction. With AST awareness, minification can be smarter: strip comment blocks, collapse multi-line parameter lists, remove blank lines between functions — all while preserving semantic correctness. Dirac's Tree-sitter queries demonstrate that structural queries are straightforward across languages.

2. **Multi-language hunk expansion:** Currently `expand.ExpandHunk` only works for Go (using `go/ast`). With Tree-sitter, the same "find enclosing function" logic extends to JS/TS, Python, Java, Swift, Rust, etc. This directly addresses the `ExpandHunk` fallback path (±N lines) that is less precise for non-Go languages.

3. **Better RAG symbol extraction:** Replace regex-based resolvers with AST-based symbol extraction. Tree-sitter can identify function calls, type references, and imported symbols with higher precision than regex, reducing both missed symbols and false matches.

4. **File skeleton extraction** (from Concept 1): Tree-sitter queries for `function_declaration`, `method_definition`, `class_declaration`, etc. work across languages with language-specific grammars.

**Implementation path:**
- Add `github.com/smacker/go-tree-sitter` (or equivalent) as a dependency. Tree-sitter grammars are available for Go, TypeScript, JavaScript, Python, Rust, Java, Swift, C, C++, C#.
- Create `cli/internal/treesitter/` package with `ParseFile(path, lang) (*tree_sitter.Tree, error)` and language detection from file extension.
- Migrate one language at a time. Start with TypeScript/JavaScript (highest impact for non-Go users).

**Relationship to existing work:**
- Directly addresses the roadmap research spike "AST-preserving minification (non-Go)."
- Consolidates the per-language RAG resolver pattern into a shared infrastructure.
- Enables Concept 1 (file skeleton) for all languages.

**Risk:** Tree-sitter Go bindings require CGO, which adds build complexity. Evaluate `github.com/tree-sitter/go-tree-sitter` (the official Go binding) vs. `github.com/smacker/go-tree-sitter` for build simplicity. If CGO is unacceptable, consider a subprocess approach (Tree-sitter CLI or a small Node helper) or continue with regex for non-Go languages.

---

## Concept 3: Compact Output Schema for Token-Constrained Local Models

### What Dirac does

Dirac's primary innovation is reducing **output tokens** (not just input). Their anchor system means the LLM never needs to reproduce existing code to locate an edit target — it references lines by short anchor words. The design principle: make the model's required output as small as possible.

### How this applies to stet

Stet focuses on reducing input tokens (minification, bounded RAG, incremental review) but hasn't optimized output tokens. The current output schema asks the model to produce a JSON array where each finding includes `file` (full path, repeated per finding), `message` (free text), `suggestion` (often reproduces code from the hunk), `category`, `severity`, `confidence`, and optional fields like `evidence_lines` and `cursor_uri`.

For local models with small context windows (8k–32k), output tokens are part of the same context budget. A verbose JSON response with 3–5 findings can consume 500–1500 tokens of output, reducing the context available for reasoning.

### Proposed adaptation

Introduce a **compact output mode** optimized for token-constrained models:

1. **Abbreviated field names:** `f` instead of `file`, `l` instead of `line`, `s` instead of `severity`, `c` instead of `category`, `m` instead of `message`, `sg` instead of `suggestion`, `cf` instead of `confidence`. The CLI maps these back to the full schema in `ParseFindingsResponse`.

2. **File path deduplication:** Since the hunk prompt already specifies `File: path/to/file.go`, the model shouldn't need to repeat it. Instruct: "The file is already specified in the prompt; omit the file field unless reporting about a different file."

3. **Line-reference suggestions:** Instead of reproducing code in the `suggestion` field, use a compact format: `"sg": "L12-L15: replace with: total := sum(items)"` referencing diff line numbers. The model outputs the delta, not the full context.

4. **Structured output / constrained decoding:** Ollama supports `format: "json"` which enables constrained decoding (the model only generates valid JSON tokens). Stet could send the JSON schema as `format` to the Ollama API, ensuring well-formed output without wasting tokens on retry/continuation when JSON is malformed. This also eliminates the `maxContinuationRounds` retry path for truncated JSON.

**Implementation path:**
- Add a `compact_output` config option (default false; true for models with context ≤ 16k).
- In `prompt.DefaultSystemPrompt`, add a compact-mode variant of the JSON schema instruction with abbreviated field names.
- In `review.ParseFindingsResponse`, add a mapping layer that normalizes compact fields back to the full `findings.Finding` struct.
- For Ollama: pass `format: { ... }` (JSON schema) in the generate request when the provider supports it, to enable constrained decoding.

**Token savings estimate:** For a typical 3-finding response:
- Full schema: ~400–600 output tokens
- Compact schema: ~200–350 output tokens
- Savings: ~30–45% of output tokens per hunk

**Relationship to existing work:**
- `cli/internal/review/parse.go` already handles JSON parsing; adding field name mapping is minimal.
- `cli/internal/ollama/client.go` already sends `format` as part of generate options; extending to schema is straightforward.
- Complements the existing `maxContinuationRounds` retry logic — with constrained decoding, truncation and malformed JSON become rare.

---

## Concept 4: Confidence-Gated Adaptive Context Expansion

### What Dirac does

Dirac implements **hierarchical context loading**: skeleton first (cheap), then specific functions (moderate), then full file (expensive) — only escalating when needed. The system prompt explicitly instructs: "LOAD INTO CONTEXT ONLY WHAT IS NECESSARY." The LLM decides what context it needs rather than being given everything upfront.

### How this applies to stet

Stet's `PrepareHunkPrompt` builds a fixed context package for every hunk: expansion (if Go) + RAG (symbol defs) + suppression examples + cursor rules. The same context budget is spent on a trivial one-line whitespace change as on a complex function rewrite. The `effectiveRAGTokenCap` is adaptive to the *remaining budget* but not to the *hunk's actual needs*.

For a 100-hunk review session, most hunks are simple (variable renames, import changes, formatting) and don't benefit from RAG or expansion context. A few hunks are complex (new logic, security-sensitive changes) and benefit greatly from rich context.

### Proposed adaptation

Implement a **two-tier review strategy** where context investment scales with hunk complexity:

**Tier 1 — Lean pass (all hunks):**
- Send hunk + minimal context (file path, user intent, suppression examples).
- No RAG, no expansion, no file skeleton.
- Token cost per hunk: ~500–2000 tokens.
- For simple hunks, this produces confident findings or clean passes quickly.

**Tier 2 — Enriched pass (complex or uncertain hunks only):**
- Triggered when Tier 1 produces findings with `confidence < 0.7` or when hunk complexity heuristics indicate the hunk warrants deeper analysis.
- Add expansion, RAG, file skeleton, call graph.
- Token cost per hunk: ~2000–8000 tokens.
- Re-review produces higher-confidence findings or validates/discards Tier 1 findings.

**Hunk complexity heuristics** (to decide whether to skip Tier 1 and go directly to Tier 2):
- Number of changed lines (> 20 → complex).
- Presence of control flow changes (if/else, switch, loop modifications).
- References to external symbols not defined in the hunk.
- Language (e.g., unsafe blocks in Rust, security-sensitive patterns).

**Implementation path:**
- Add `adaptive_context` config option (default false for v1; opt-in).
- In the run loop (`cli/internal/run/run.go`), when `adaptive_context` is true: build lean prompt first, call LLM, examine findings. If any finding has `confidence < threshold`, rebuild with enriched prompt and re-call LLM for that hunk.
- Track token savings: log total tokens used with adaptive vs. what would have been used with full context for every hunk. Report in `stet stats`.

**Token savings estimate:** In a typical review session:
- ~60–70% of hunks are simple and don't need enriched context.
- Per-hunk savings on simple hunks: ~1000–4000 tokens (RAG + expansion skipped).
- For a 50-hunk session: ~30,000–100,000 tokens saved overall.
- Trade-off: ~5–10% of hunks may need a second LLM call (2× cost for those hunks), but total session cost still drops significantly.

**Relationship to existing work:**
- The existing `effectiveRAGTokenCap` budgeting logic provides the infrastructure — this concept adds a *decision layer* above it.
- The critic (`cli/internal/review/critic.go`) is already a "second pass" concept — adaptive context is the inverse: the *first* pass is lean, and the second pass adds context rather than adding verification.
- Aligns with `docs/defect-focused-review-plan.md` Phase 3 (Self-Critique) — confidence scores are already in the schema and can drive the adaptive decision.

---

## Summary: Concepts Ranked by Impact and Feasibility

| # | Concept | Token Impact | Accuracy Impact | Feasibility | Stet Roadmap Alignment |
|---|---------|-------------|----------------|-------------|----------------------|
| 1 | File Skeleton Context | Low cost (~100 tokens/hunk) | High (reduces "undefined symbol" FPs, structural awareness) | High (Go: use go/ast; others: regex fallback) | Extends Tier 2 in context-enrichment-research.md |
| 2 | Tree-sitter Universal AST | Enables 10–30% minification for non-Go | Medium (better symbol extraction, expansion) | Medium (CGO dependency, per-language grammars) | Directly addresses roadmap research spike |
| 3 | Compact Output Schema | 30–45% output token reduction | Medium (constrained decoding reduces malformed JSON) | High (schema mapping + Ollama format param) | New; complements existing parse/retry logic |
| 4 | Adaptive Context Expansion | 30–50% total session token reduction | High (more context where it matters, less noise elsewhere) | Medium (requires confidence-gated re-review logic) | Extends effectiveRAGTokenCap and critic concepts |

### Recommended adoption order

1. **Concept 3 (Compact Output)** — Highest feasibility, immediate token savings, minimal risk. Can be shipped as a config flag with zero impact on existing behavior.
2. **Concept 1 (File Skeleton)** — High value-to-effort ratio. Go implementation uses existing `go/ast`; provides structural context that directly reduces a known class of false positives.
3. **Concept 4 (Adaptive Context)** — Largest potential token savings but requires more infrastructure. Implement after Concepts 1 and 3 provide the "enriched" and "lean" prompt variants that the adaptive system switches between.
4. **Concept 2 (Tree-sitter)** — Strategic infrastructure investment. Defer until Concepts 1, 3, 4 prove the value of multi-language AST operations, then consolidate under Tree-sitter.

---

## References

- [Dirac repository](https://github.com/dirac-run/dirac) — Apache 2.0, TypeScript
- [Dirac Terminal-Bench-2 results](https://github.com/dirac-run/dirac#benchmarks) — 65.2% score, 64.8% cost reduction
- Stet docs: [roadmap.md](roadmap.md), [context-enrichment-research.md](context-enrichment-research.md), [defect-focused-review-plan.md](defect-focused-review-plan.md), [review-quality.md](review-quality.md)
- Tree-sitter Go bindings: [go-tree-sitter](https://github.com/smacker/go-tree-sitter), [official go-tree-sitter](https://github.com/tree-sitter/go-tree-sitter)
