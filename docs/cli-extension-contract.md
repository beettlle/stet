# CLI–Extension Contract

This document defines the output and exit-code contract between the Stet CLI and the Cursor extension (Phase 5). The extension spawns the CLI and parses its output.

## Commands that emit findings

- **`stet start [ref]`** — On success, writes findings JSON to stdout.
- **`stet run`** — On success, writes findings JSON to stdout.

## stdout

On success (exit code 0), the CLI writes exactly one JSON object, followed by a newline:

```json
{"findings": [ ... ]}
```

- **`findings`**: Array of finding objects. May be empty (e.g. nothing to review).
- Each element has the following fields (see `cli/internal/findings/finding.go` for the canonical schema):
  - **`id`** (string, optional): Stable identifier for the finding.
  - **`file`** (string): Relative file path.
  - **`line`** (number, optional): Line number.
  - **`range`** (object, optional): `{"start": n, "end": m}` for multi-line span.
  - **`severity`** (string): `"error"`, `"warning"`, `"info"`, or `"nitpick"`.
  - **`category`** (string): e.g. `"bug"`, `"security"`, `"style"`, `"maintainability"`, `"performance"`, `"testing"`.
  - **`message`** (string): Description of the finding.
  - **`suggestion`** (string, optional): Suggested fix.
  - **`cursor_uri`** (string, optional): Deep link (e.g. `file://` or `cursor://`).

## stderr

Human-readable error and diagnostic messages. The extension should surface these when the process exits with a non-zero code. For some conditions (e.g. uncommitted changes, or an existing review session), the CLI also prints a one-line **recovery hint** (e.g. `Hint: Run 'stet finish'...`) so the extension or user can suggest the next command.

### Recovery hints

On `stet start` failure, the CLI may print one of the following hints to stderr before the error line:

| Condition | Hint |
|-----------|------|
| Uncommitted changes | `Hint: Commit or stash your changes, then run 'stet start' again.` |
| Worktree already exists | `Hint: Run 'stet finish' to end the current review and remove the worktree, then run 'stet start' again.` |

## Exit codes

| Code | Meaning |
|------|--------|
| **0** | Success; stdout contains the findings JSON. |
| **1** | Usage error or other failure (e.g. not a git repo, no session, model not found). |
| **2** | Ollama unreachable (server not running or not reachable). |

## Working directory

The CLI must be run from the repository root (or from a directory under the repo) so that `git rev-parse --show-toplevel` succeeds. Invoke from repo root (e.g. `cd /path/to/repo && stet start --dry-run`).

## Cleanup after interrupted runs

If multiple stet worktrees remain after interrupted runs (e.g. `git worktree list` shows entries under `.review/worktrees/stet-*`), run `stet finish` to remove the current session's worktree, then remove any remaining paths with `git worktree remove <path>`. Phase 6 will add an optional `stet cleanup` command for orphan worktrees.

## Environment and pipelines

For pipelines or multiple commands (e.g. `stet doctor ; stet start`), `STET_OLLAMA_BASE_URL` and other `STET_*` variables must be **exported** (or set in the shell before both commands) so every `stet` invocation sees the same config. Command-prefixed env (e.g. `VAR=value cmd1 ; cmd2`) only applies to the first command; the second process will not see that variable and may fall back to defaults (e.g. `http://localhost:11434`), which can cause "Ollama unreachable" even when the first command succeeded.

## Usage (extension)

1. Spawn the CLI with the desired subcommand and args (e.g. `stet start --dry-run` or `stet run`), from the repository root.
2. On exit code 0: read stdout and parse the single JSON object; use `findings` to populate the panel.
3. On non-zero exit: read stderr for the error message; use exit code 2 to show a specific “Ollama unreachable” message if desired.
