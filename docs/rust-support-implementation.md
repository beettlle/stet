# Rust support implementation (Phase 1 and Phase 2)

This document specifies the implementation of Rust support for stet so that `.rs` files get correct hunk identity, RAG symbol lookup, optional minification, Cursor rules, and documentation. Phase 3 (expansion, call-graph) is out of scope.

---

## Summary

| Phase | Deliverable        | Primary files                                          | Acceptance                                       |
| ----- | ------------------ | ------------------------------------------------------ | ------------------------------------------------ |
| 1     | Hunk ID Rust       | `hunkid/hunkid.go`, `hunkid_test.go`                   | `go test ./internal/hunkid/...`                  |
| 1     | RAG Rust resolver  | `rag/rust/resolver.go`, `main.go`                      | `go build ./...`; `go test ./internal/rag/...`   |
| 1     | AGENTS.md Rust     | `AGENTS.md`                                            | Manual                                           |
| 2     | Minify Rust        | `minify/minify.go`, `review/review.go`                 | `go test ./internal/minify/...`                  |

All paths under Primary files are relative to `cli/internal/` except `AGENTS.md` (repo root).

---

## Phase 1 – Minimal Rust support

Phase 1 delivers: semantic hunk ID for `.rs`, RAG Rust resolver, Cursor rules wiring, and AGENTS.md update.

### 2.1 Hunk ID – semantic hash for Rust

**Goal:** `SemanticHunkID` for `.rs` files strips comments and collapses whitespace so "already reviewed" matching works for Rust. Rust uses `//`, `/* */`, `///`, `//!`; reuse Go-style stripping.

**File:** `cli/internal/hunkid/hunkid.go`

- In `langFromPath`: add `case ".rs": return "rust"`.
- In `codeOnly`: add `case "rust": stripped = stripGoStyleComments(content)` (reuse existing helper; no new function).

**File:** `cli/internal/hunkid/hunkid_test.go`

- Add `TestLangFromPath` entry for `"x.rs"` → `"rust"`.
- Add `TestSemanticHunkID_Rust`: at least two rows — (1) `src/lib.rs` with `fn f() { }`, (2) same content with `// comment` or extra whitespace; assert same semantic ID, different strict ID where applicable.

**Acceptance:** `cd cli && go test ./internal/hunkid/... -count=1`

### 2.2 RAG – Rust symbol resolver

**Goal:** New resolver that implements `rag.Resolver` for `.rs`: extract symbols from hunk (fn, struct, enum, trait, type names, call sites), look up definitions via `git grep`, return `rag.Definition` (Symbol, File, Line, Signature, Docstring).

**New package:** `cli/internal/rag/rust/resolver.go`

- **Pattern:** Mirror `cli/internal/rag/go/resolver.go`: `Resolver` struct, `init()` with `rag.MustRegisterResolver(".rs", New())`, `ResolveSymbols(ctx, repoRoot, filePath, hunkContent, opts)`.
- **Symbol extraction:** Regexes for: `fn\s+(\w+)`, `struct\s+(\w+)`, `enum\s+(\w+)`, `trait\s+(\w+)`, `impl\s+.*?\s+for\s+(\w+)`, `(\w+)\s*\(` (call sites). Filter with a `rustKeywords` map (e.g. `fn`, `struct`, `enum`, `trait`, `impl`, `match`, `let`, `mut`, `pub`, `use`, `mod`, `async`, `move`, `self`, `Self`, `true`, `false`, `type`, `where`, etc.).
- **Lookup:** `git grep -n -E` with POSIX `[[:space:]]` patterns for definitions, e.g. `fn\s+Symbol\s*[<(]`, `struct\s+Symbol\s*[{]`, `enum\s+Symbol\s*[{]`, `trait\s+Symbol\s*[{]`. Use `grepTimeout` (e.g. 5s), `maxSymbolCandidates` (e.g. 30), `maxPrecedingCommentLines` (e.g. 5). Reuse `minimalEnv(repoRoot)` pattern from Go/JS for `GIT_DIR`/`GIT_WORK_TREE`.
- **Signature and docstring:** Read file at matched path/line; signature = declaration line(s) up to `{` or `;`; docstring = preceding `///`, `//!`, or `/** */` lines (same approach as JS resolver).
- **Token capping:** If `opts.MaxTokens > 0`, cap definitions with a `capDefinitionsByTokens` helper (same pattern as Go/JS).

**Wiring:**

- `cli/cmd/stet/main.go` — add blank import: `_ "stet/cli/internal/rag/rust"`.
- `cli/internal/review/review_test.go` — add same import so RAG tests can resolve Rust when needed.

**Tests:** `cli/internal/rag/rust/resolver_test.go`

- Unit: `extractSymbols` (or equivalent) — given hunk snippet containing `foo()`, `Bar`, `baz`, assert extracted list contains expected symbols and excludes keywords.
- Optional: integration test with temp repo containing a Rust file with `fn foo()` and a hunk that calls `foo`; `ResolveSymbols` returns one definition with correct File/Line/Signature.

**Acceptance:** `cd cli && go build ./...` and `go test ./internal/rag/... -count=1`

### 2.3 Cursor rules and AGENTS.md

**Cursor rules:** The rules loader (`cli/internal/rules`) is glob-based; no code change. The **project** (or stet repo) should include Rust-targeted rule files in `.cursor/rules/`, e.g.:

- `rust-development-standards.mdc` with frontmatter `globs: "*.rs"` (and optional `description`).
- `rust-brutal-audit.mdc` with `globs: "*.rs"` for phase-verification steps.

**File:** `AGENTS.md`

- Under "Language-Specific Standards", add a **Rust** subsection after JavaScript/TypeScript: list Standards and Audit rule file names (e.g. `rust-development-standards.mdc`, `rust-brutal-audit.mdc`).
- Under "Phase Verification (Brutal Audit)", add: "For Rust code: run the steps in `.cursor/rules/rust-brutal-audit.mdc`."

**Acceptance:** No automated acceptance; manual check that AGENTS.md contains Rust section and that adding a rule file with `globs: "*.rs"` in a repo causes stet to inject it when reviewing `.rs` files.

### Phase 1 exit criteria

- All Phase 1 acceptance commands pass.
- Semantic hunk ID for `.rs` matches comment/whitespace-insensitive behavior.
- RAG resolves symbols for `.rs` when resolver is registered and definitions exist.
- AGENTS.md documents Rust standards and audit.

---

## Phase 2 – Minify and tests

Phase 2 adds: minification of Rust hunks (token saving) and any remaining tests.

### 3.1 Minify – Rust hunk content

**Goal:** Apply the same per-line whitespace reduction as Go to Rust unified-diff hunks (preserve `@@` header and line prefix space/`-`/`+`; trim/collapse spaces in the rest). Safe for Rust because semantics are unchanged.

**File:** `cli/internal/minify/minify.go`

- Add `MinifyRustHunkContent(content string) string` with the same logic as `MinifyGoHunkContent` (or refactor to a shared `minifyUnifiedHunkContent(content string) string` and call it from both `MinifyGoHunkContent` and `MinifyRustHunkContent` to avoid duplication).

**File:** `cli/internal/review/review.go`

- Where minify is applied (around line 117): extend condition from `filepath.Ext(hunk.FilePath) == ".go"` to also allow `.rs`, and call `minify.MinifyRustHunkContent` for `.rs` (or the shared helper if refactored).

**File:** `cli/internal/minify/minify_test.go`

- Add test case(s) for Rust: e.g. a unified-diff hunk with `fn foo() { ... }` and varying leading/collapsible spaces; assert output is minified and header/prefix preserved (mirror `TestMinifyGoHunkContent`).

**Acceptance:** `cd cli && go test ./internal/minify/... ./internal/review/... -count=1`

### 3.2 Test coverage and regression

- Ensure Phase 1 tests remain passing; add any missing Rust RAG tests (e.g. `TestResolveSymbols_rust_returnsDefinitions` in rag test or in `rust/resolver_test.go`).
- Coverage gates (77% project, 72% per file) apply to new/changed code per [AGENTS.md](AGENTS.md).

### Phase 2 exit criteria

- All Phase 2 acceptance commands pass.
- Rust hunks are minified when review runs; token usage for Rust diffs is reduced.
- No regression in Phase 1 behavior.

---

## Out of scope

- **Phase 3:** Hunk expansion for Rust (tree-sitter or heuristic), call-graph resolver for Rust. See [docs/roadmap.md](docs/roadmap.md) or implementation-plan for future "AST-preserving minification (non-Go)" / expansion.
- **Dev container:** Adding Rust toolchain to `.devcontainer` is optional and not required for Phase 1 or 2.
