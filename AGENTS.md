# AI Agent Guide (AGENTS.md)

This project uses **CursorRules** to define coding standards, anti-patterns, and workflows. The full rules are located in the `.cursor/rules/` directory.

## 🚫 Universal Anti-Patterns

The following are the Top 15 Universal Anti-Patterns from `.cursor/rules/critical-rules-quick-reference.mdc`. Avoid these at all costs.

### 1. Ghost Layer Prevention

❌ Bad: `class Service { func get() { return repo.get() } }` (>80% delegation)
✅ Good: Service coordinates multiple repos OR adds logic OR delete layer
⚠️ Why: Adds complexity without value, violates SRP
📍 Details: `general-llm-anti-patterns.mdc` section 1.1

### 2. Fake Abstraction Patterns

❌ Bad: `actor Repo { func process() async { await MainActor.run { } } }` (defeats isolation)
✅ Good: `actor Repo { func process() async { /* actual async work */ } }`
⚠️ Why: Creates illusion of concurrency without benefits, adds overhead
📍 Details: `general-llm-anti-patterns.mdc` section 1.2

### 3. Placeholder & Dead Code Ban

❌ Bad: `func method() { throw NotImplementedError() }` or `_ = expensiveOperation()`
✅ Good: Implement, remove, or mark unavailable with explanation
⚠️ Why: Broken functionality, wasted resources, technical debt
📍 Details: `general-llm-anti-patterns.mdc` section 1.3

### 4. Business Logic Location

❌ Bad: Repository contains switch statements, state transitions, business calculations
✅ Good: Repository is pure CRUD, business logic in Domain layer
⚠️ Why: Creates God Objects, untestable code, tight coupling
📍 Details: `general-llm-anti-patterns.mdc` section 1.4

### 5. Performance-First Collection Handling

❌ Bad: `for item in items { await processItem(item) }` (I/O in loop = O(N²))
✅ Good: Batch operations - fetch once, modify in-memory, save once
⚠️ Why: UI freezes at scale, database exhaustion, poor UX
📍 Details: `general-llm-anti-patterns.mdc` section 2.1

### 6. Zero-Hallucination Policy

❌ Bad: Multi-step search workaround when direct lookup API exists
✅ Good: Verify framework documentation, use official APIs
⚠️ Why: Fragile code, security risk (slopsquatting), build failures
📍 Details: `general-llm-anti-patterns.mdc` section 4.1

### 7. Hardcoded Secrets

❌ Bad: `apiKey = "sk-live-1234567890abcdef"` in source code
✅ Good: `apiKey = process.env.API_KEY` (environment variables)
⚠️ Why: Security vulnerability, credential exposure, compliance violations
📍 Details: `general-llm-anti-patterns.mdc` section 5.1

### 8. Missing Input Validation

❌ Bad: `database.query("SELECT * FROM users WHERE id = " + userInput)`
✅ Good: Validate input, use parameterized queries
⚠️ Why: SQL injection, XSS, command injection, system compromise
📍 Details: `general-llm-anti-patterns.mdc` section 5.2

### 9. Silent Failures

❌ Bad: `try { op() } catch { pass }` (swallows errors)
✅ Good: Handle specific errors or explicitly propagate with context
⚠️ Why: Hidden bugs, difficult debugging, silent data loss
📍 Details: `general-llm-anti-patterns.mdc` section 5.3

### 10. Test-Implementation Mismatch

❌ Bad: Mock returns different types/behavior than real implementation
✅ Good: Mocks match real API signatures and behavior exactly
⚠️ Why: False confidence, integration failures, production bugs
📍 Details: `general-llm-anti-patterns.mdc` section 6.1

### 11. Warning Dismissal Anti-Pattern

❌ Bad: Comments dismissing warnings as "false positives" or "won't affect runtime"
✅ Good: Fix all warnings; if truly unfixable, document technical reason and escalate
⚠️ Why: Warnings indicate real issues; dismissing them creates technical debt
📍 Details: `general-llm-anti-patterns.mdc` section 3.7

### 12. False Compilation Claims

❌ Bad: "All code compiles with zero warnings" (without running build)
✅ Good: "Code changes complete. Build verification pending." OR show actual build output
⚠️ **STOP:** If typing "compiles"/"builds"/"zero warnings" → Verify: Did I run a build? If no → Use "Build verification pending"
⚠️ Why: Creates false confidence, wastes user time, violates trust, leads to broken code
📍 Details: `general-llm-anti-patterns.mdc` section 3.8

### 13. False Test Verification Claims

❌ Bad: "All tests pass" (without running tests)
✅ Good: "Code changes complete. Test verification pending." OR show actual test output
⚠️ Why: Creates false confidence, wastes user time, violates trust, leads to broken code
📍 Details: `general-llm-anti-patterns.mdc` section 3.9

### 14. Package Verification (Slopsquatting Prevention)

❌ Bad: `import suspicious_package` (unverified package), importing plausible-sounding packages without checking
✅ Good: Verify package exists in registry, check maintainer, check download stats, verify owner
⚠️ Why: Security risk (slopsquatting), attackers register packages with LLM-hallucinated names, build failures
🔧 Fix: Always verify packages exist, check package registry before importing, verify maintainer identity
📍 Details: `general-llm-anti-patterns.mdc` section 4.2d

### 15. Example Over-Reliance

❌ Bad: Example code used verbatim without adaptation
✅ Good: Adapt examples to project patterns, verify examples match requirements
⚠️ Why: Example code may not fit project patterns, introduces inconsistencies
📍 Details: `general-llm-anti-patterns.mdc` section 4.4

## 📚 Language-Specific Standards

This project utilizes **Go** and **JavaScript/TypeScript**. Refer to the following rules for detailed standards:

### Go
- **Standards:** `.cursor/rules/go-1-21-development-standards.mdc`
- **Audit:** `.cursor/rules/go-1-21-brutal-audit.mdc`

### JavaScript/TypeScript
- **Standards:** `.cursor/rules/javascript-3-development-standards.mdc`
- **Audit:** `.cursor/rules/javascript-3-brutal-audit.mdc`

### Rust
- **Standards:** `.cursor/rules/rust-development-standards.mdc`
- **Audit:** `.cursor/rules/rust-brutal-audit.mdc`

## 🛠️ Project-Specific Instructions

### Development Environment
- **Dev Container:** The project includes a `.devcontainer` configuration. Used for a consistent environment.
- **Languages:**
  - **Go:** 1.22 or later.
  - **Node.js/TypeScript:** Required for the extension.
- **Directory Structure:**
  - `cli/`: Go source code (main module).
  - `extension/`: TypeScript source for the Cursor/VSCode extension.
  - `docs/`: Project documentation.
- **LLM backends:** Default is **Ollama** (`provider` / `STET_PROVIDER`). Optional **OpenAI-compatible** HTTP API (`openai_base_url` / `STET_OPENAI_BASE_URL`, `max_completion_tokens` / `STET_MAX_COMPLETION_TOKENS`). See [README.md](README.md) for a short user overview and [docs/cli-extension-contract.md](docs/cli-extension-contract.md) for the full configuration contract and precedence.

### Build and Run
- **CLI (Go)**:
  ```bash
  cd cli
  go build ./...
  ```
- **Extension (TypeScript)**:
  ```bash
  cd extension
  npm install
  npm run compile
  ```

### Testing
**Strict Coverage Requirements:**
- **77%** line coverage for the **entire project**.
- **72%** minimum line coverage for **every file**.
- **Run Tests:**
  - Go: `go test ./... -cover`
  - Extension: `npm test` (Vitest)

### PR and Commit Guidelines
- **Source of Truth:** All implementation MUST follow `docs/PRD.md` and `docs/implementation-plan.md`.
- **Phased Delivery:** Complete work in phases as defined in the Implementation Plan.
- **Verification:** A phase is not complete until new/changed code has tests, coverage thresholds are met, and existing tests pass.

### Known acceptable warnings (builds and tests still pass)
- **Extension `npm install`:** Deprecated `glob` warning from transitive deps (Vitest coverage). Resolving would require Vitest 4 or glob overrides that need Node 20+.
- **Extension `npm test`:** "The CJS build of Vite's Node API is deprecated" comes from Vitest; tests pass. Upgrading to Vitest 4 would address it.
- **Extension `npm test`:** One test intentionally triggers an error path; stderr "Failed to clear findings panel" in `finishReview.test.ts` is expected and not a failure.

## 🔄 Phase Verification (Brutal Audit)

**Before marking a phase or task as complete, you MUST run the appropriate "Brutal Audit" to verify quality.**

- **For Go code:** Run the steps in `.cursor/rules/go-1-21-brutal-audit.mdc`.
- **For JS/TS code:** Run the steps in `.cursor/rules/javascript-3-brutal-audit.mdc`.
- **For Rust code:** Run the steps in `.cursor/rules/rust-brutal-audit.mdc`.

**To invoke:** Ask the agent to "Run the brutal audit" or "Verify this phase".
