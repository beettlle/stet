# Publish failure recovery and post-release smoke

Read only when `release.yml` fails or the smoke test fails. The happy path is in [SKILL.md](../SKILL.md) Phase 6.

## Rule zero

**Never move, delete, or re-push a tag that is already on `origin`.** Users and `releases/latest/download` may already resolve it. Recovery is either a re-dispatch of the same tag or a new patch version.

## Diagnose first

```bash
gh run list --workflow release.yml --limit 5
gh run view <run-id> --log-failed
gh release view vX.Y.Z --json isDraft,assets --jq '{draft: .isDraft, assets: [.assets[].name]}'
```

## Symptom table

| Symptom | Published? | Action |
|---------|-----------|--------|
| `Validate tag` failed | No | Tag name is not `vX.Y.Z`. Leave the bad tag; tag the correct version |
| `Release check` failed (gofmt, vet, tests, coverage, extension, cross-compile) | No | The tagged commit is not release-safe. Fix on `main`, run the full Phase 5 gate, release the **next patch** version. Record the dead tag in the new release notes |
| `Build release binaries` / `Check embedded version` failed | No | Same as above: fix on `main`, next patch version |
| `Publish GitHub Release` failed (network, API 5xx, rate limit) | Maybe partially | Re-dispatch the same tag: `gh workflow run release.yml -f tag=vX.Y.Z`. The job re-uploads assets with `--clobber` when the release already exists |
| Release exists but assets are missing or stale | Partially | Re-dispatch the same tag (as above) |
| Release body is wrong | Yes | Edit notes only: `gh release edit vX.Y.Z --notes-file <fixed-notes.md>`. The tag message stays as is |
| `verify-install` failed on one OS, assets look right | Yes | Read that job's log. Transient download failure: re-run the failed job (`gh run rerun <run-id> --failed`). Installer or binary bug: fix forward with the next patch version and mention the broken version in its notes |
| `scripts/post-release-smoke.sh` fails on the asset check | Yes | Re-dispatch the same tag, then re-run the smoke test |
| Smoke install fails while `verify-install` passed | Yes | Usually `raw.githubusercontent.com` caching an older `install.sh`; the script already retries with backoff. If retries are exhausted, run the installer manually with `STET_DOWNLOAD_BASE_URL` pointing at `https://github.com/beettlle/stet/releases/download/vX.Y.Z` to separate installer-cache issues from asset issues |

## Re-dispatch an existing tag

```bash
gh workflow run release.yml -f tag=vX.Y.Z
sleep 5
gh run list --workflow release.yml --event workflow_dispatch --limit 3
gh run watch --exit-status <run-id>
```

The dispatched run uses the workflow file from `main` but checks out the tag, so the gate and build run against the tagged code. Tags that predate `scripts/release-check.sh` (only `v1.0.0`) fall back to `make test`.

Rebuilt binaries are byte-identical only if the Go patch release is unchanged; `--clobber` replaces binaries and `checksums.txt` together, so installers stay consistent.

## Post-release smoke

```bash
scripts/post-release-smoke.sh vX.Y.Z
```

It checks the release is not a draft and has exactly the nine expected assets, then installs with the public one-liner (`STET_RELEASE_TAG=vX.Y.Z`) into a temp directory and asserts `stet --version` reports the tag. Up to six attempts with 5s/10s/20s/40s/60s backoff; exhausted retries exit 1. **STOP** and diagnose on failure; never report the release as shipped without a passing smoke run.
