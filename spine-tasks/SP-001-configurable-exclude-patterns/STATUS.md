**Current Step:** Step 0: Design and discovery
**Status:** Ready
**Last Updated:** 2026-06-30
**Review Level:** 2
**Review Counter:** 0
**Iteration:** 0
**Size:** M

---

## Step 0: Design and discovery

**Status:** Not Started

- [ ] Read issue #1 and context files listed in PROMPT
- [ ] Confirm replace-vs-merge behavior in current `diff.go` and record decision in Discoveries
- [ ] Choose `STET_EXCLUDE_PATTERNS` env convention
- [ ] List all `scope.Partition` call sites to wire

## Step 1: Config, env, and diff merge semantics

**Status:** Not Started

- [ ] Add config fields and env loading for exclude patterns
- [ ] Implement merge vs replace in diff layer
- [ ] Add config and diff unit tests

## Step 2: CLI flags and run wiring

**Status:** Not Started

- [ ] Add `--exclude` and `--no-exclude` flags to start/run/rerun
- [ ] Pass resolved `*diff.Options` into `scope.Partition`
- [ ] Verify incremental review when exclusions change
- [ ] Add dry-run integration test

## Step 3: Observability and documentation

**Status:** Not Started

- [ ] Log skipped paths on stderr (verbose/trace)
- [ ] Update `docs/cli-extension-contract.md`
- [ ] Update `docs/review-process-internals.md`

## Step 4: Testing and verification

**Status:** Not Started

- [ ] Run `cd cli && go test ./... -cover`
- [ ] Verify coverage thresholds per `AGENTS.md`
- [ ] Fix all introduced failures

---

## Reviews

| Date | Step | Type | Outcome |
|------|------|------|---------|
| | | | |

## Discoveries

| Date | Finding | Impact |
|------|---------|--------|
| | | |

## Execution Log

| Date | Event | Detail |
|------|-------|--------|
| | | |

## Blockers

| Date | Blocker | Resolution |
|------|---------|------------|
| | | |

## Notes

- Source: [GitHub issue #1](https://github.com/beettlle/stet/issues/1)
