# SP-062: `main_test.go`: one output-capture helper that drains and closes pipes — Status

**Current Step:** Not Started
**Status:** ⬜ Not Started
**Last Updated:** 2026-10-22
**Review Level:** 1
**Review Counter:** 0
**Iteration:** 0
**Size:** M

---

### Step 1: Capture helper
**Status:** ⬜ Not Started

- [ ] Helper drains while running, restores via `defer`, closes all pipe ends
- [ ] Helper tests (>128 KiB, separate streams, restore)
- [ ] `runCLICapture` uses the helper

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
