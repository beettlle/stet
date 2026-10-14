# Issue intake checklist

Run in **Phase 1** before composing the manifest. The snapshot script collects everything below in one pass:

```bash
.agents/skills/stet-release-operator/scripts/collect-release-intake.sh {TARGET}
```

## GitHub queries

Repo: `beettlle/stet`. Shortlist by title and labels; read bodies only for candidates (bodies for the whole backlog waste context).

```bash
gh issue list --repo beettlle/stet --state open --limit 100 --json number,title,labels
gh issue view NNN --repo beettlle/stet --json number,title,labels,body
```

## Label taxonomy

| Label | Bucket | Notes |
|-------|--------|-------|
| `documentation` | Documentation | Before enhancements |
| `bug` | Bug fix | Prefer reproducible, user-visible, S-sized |
| `enhancement` | Enhancement | 0 per patch, 1–2 per minor |
| `good first issue` / `help wanted` | — | Informational; no priority change |
| `wontfix` / `invalid` / `duplicate` / `question` | — | Exclude |

## Issue → task mapping

```bash
rg -n 'Closes:? #[0-9]+|Partial:? #[0-9]+' spine-tasks/*/PROMPT.md
```

| State | Action |
|-------|--------|
| Mapped to pending SP-* | Candidate if it fits the profile |
| Mapped to `.DONE` SP-* but issue still OPEN | Hygiene debt: close it now with a comment citing the SP-* and commit; do not select new work for it |
| No SP-* | **Gap**: author with `create-spine-tasks` in Phase 3 |
| Epic or multi-area | Defer unless major profile with operator approval |

## Bug heuristics

Prefer when:

- The issue has repro steps or a named failing command (`stet run`, `stet status`, `stet benchmark`, ...)
- The fix is confined to one `cli/internal/<pkg>` (disjoint file scope from other wave tasks)
- It affects review correctness, dismissals, session state, or token cost

Exclude when the repro needs an external model server behavior that cannot be faked in tests (record "needs manual repro" and defer).

## Documentation heuristics

- `label:documentation` or docs-only pending SP-* (File Scope under `docs/`, `README.md`, `AGENTS.md`)
- Every doc path in Documentation Requirements must also appear in File Scope
- Behavior changes in a bug task that alter documented CLI output must update `docs/cli-extension-contract.md` in the same task

## Dependency drift check (every profile)

Always record results; include a task only when a threshold fires.

```bash
go list -u -m -f '{{if .Update}}{{.Path}} {{.Version}} -> {{.Update.Version}}{{if .Indirect}} (indirect){{end}}{{end}}' all
govulncheck ./cli/...        # or: go run golang.org/x/vuln/cmd/govulncheck@latest ./cli/...
rg -n 'go-version:|node-version:|uses: ' .github/workflows/
(cd extension && npm outdated; npm audit --omit=dev)
```

| Signal | Include? | Bucket |
|--------|----------|--------|
| `govulncheck` finding on a **called** symbol (reachable) | Yes — even in patch | bug |
| `govulncheck` finding in an imported but uncalled module | Minor/major only | enhancement / hygiene |
| Direct dependency ≥1 minor behind | Minor/major only, if a slot is free | enhancement / hygiene |
| Indirect-only drift | Defer; record | — |
| Go toolchain pin (`1.27.x` in workflows) behind the latest supported Go release | Minor/major only | enhancement / hygiene |
| GitHub Actions major behind | Minor/major only | enhancement / hygiene |
| Extension npm drift or audit findings | Defer — extension is not shipped | — |
| No threshold fires | Record "deps OK — deferred"; no task | — |

Prefer one focused dependency packet over "bump everything".

## Intake output (required)

| Issue # | Labels | Mapped SP-* | Bucket | Profile fit | Notes |
|---------|--------|-------------|--------|-------------|-------|
| #4 | bug | — (gap) | bug | patch ✓ | S; `cli/internal/benchmark` |

Dependency mini-table:

| Module / tool | Current | Latest | Action |
|---------------|---------|--------|--------|
| (from `go list -u -m`) | … | … | include SP-### / defer: reason |
| govulncheck | — | — | N reachable findings: include / none |
