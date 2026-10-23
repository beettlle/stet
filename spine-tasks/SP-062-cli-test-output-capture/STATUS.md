# SP-062: `main_test.go`: one output-capture helper that drains and closes pipes — Status

**Current Step:** Step 2
**Status:** 🔄 In Progress — Step 1 complete
**Last Updated:** 2026-10-23
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** M

---

### Step 1: Capture helper
**Status:** ✅ Complete

- [x] Helper drains while running, restores via `defer`, closes all pipe ends
- [x] Helper tests (>128 KiB, separate streams, restore)
- [x] `runCLICapture` uses the helper

---

### Step 2: Migrate captures in main_test.go
**Status:** ⬜ Not Started
> ⚠️ Hydrate: list migrated test names in batches as you go

- [ ] All `os.Pipe()` sites in `main_test.go` migrated

---

### Step 3: Testing & Verification
**Status:** ⬜ Not Started

- [ ] Coverage before/after recorded; after ≥ before
- [ ] No `os.Pipe()` in `main_test.go`
- [ ] Contract `testCommand` passing

---

## Reviews

| # | Type | Step | Verdict | File |
|---|------|------|---------|------|

---

## Discoveries

| Discovery | Disposition | Location |
|-----------|-------------|----------|
| Initial helper returned `stdoutBuf.String()` at a plain `return`, which evaluates before the deferred `drains.Wait()` — captured output was truncated (lost exactly what was still in flight). Fixed with named returns assigned inside the deferred cleanup after drains finish. | Fixed in `captureOutput` | `cli/cmd/stet/capture_test.go` |
| `os.Pipe()` also exists in `cli/internal/git/ignore_test.go` and `cli/internal/run/*_test.go` — outside this task's File Scope, left untouched (the "os.Pipe only in capture_test.go" criterion is met within `cli/cmd/stet`). | Logged | — |
