# CLI–Extension Contract

This document defines the output and exit-code contract between the Stet CLI and the Cursor extension (Phase 5). The extension spawns the CLI and parses its output.

## Commands that emit findings

- **`stet start [ref]`** — On success, writes findings JSON to stdout.
- **`stet run`** — On success, writes findings JSON to stdout.
- The **`--dry-run`** flag skips the LLM and emits deterministic findings for CI.

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
  - **`cursor_uri`** (string, optional): Deep link (e.g. `file://` or `cursor://`). When the CLI sets it (when the model omits it), it uses `file://` with absolute path and line (or range) so the extension can open at location.

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

## Other commands

- **`stet status`** — Reports baseline, last_reviewed_at, worktree path, finding count, and dismissed count. Exits 1 with "No active session" if no session.
- **`stet approve <id>`** — Adds the finding ID to the session’s dismissed list so it does not resurface in findings output. Idempotent. Exits 1 if no active session.
- **`stet finish`** — Ends the session and removes the worktree. Exits 1 if no active session.

## Optimizer (stet optimize)

The optional **`stet optimize`** command invokes an external script (e.g. a Python DSPy optimizer) to improve the system prompt from user feedback. The Go CLI has **no Python or DSPy dependency**; it only runs the configured command.

- **When to run**: e.g. weekly or after enough feedback has been collected in `.review/history.jsonl`.
- **Input**: The script reads `.review/history.jsonl` (see State storage and history below). The CLI passes the state directory via the **`STET_STATE_DIR`** environment variable when invoking the script.
- **Output**: The script should write `.review/system_prompt_optimized.txt`. When that file exists, the CLI uses it as the system prompt for review (see Phase 3.3).
- **Configuration**: Set the command to run via **`STET_OPTIMIZER_SCRIPT`** or **`optimizer_script`** in repo/global config (e.g. `python3 scripts/optimize.py` or a path to your script). If unset, `stet optimize` exits 1 with a message asking you to configure it.
- **Exit codes**: 0 = success; non-zero = failure (script missing, Python/DSPy error, invalid history, etc.). The CLI propagates the script’s exit code when in 0–255.

## Configuration (Ollama model options)

The CLI passes model runtime options to the Ollama API on each generate request. Config file keys and environment variables:

| Key / env | Default | Description |
|-----------|---------|-------------|
| `temperature` / `STET_TEMPERATURE` | 0.2 | Sampling temperature (0–2). Lower values give more deterministic output. |
| `num_ctx` / `STET_NUM_CTX` | 32768 | Model context window size (tokens). Set to 0 in config/env to use default. |
| `optimizer_script` / `STET_OPTIMIZER_SCRIPT` | (none) | Command to run for `stet optimize` (e.g. `python3 scripts/optimize.py`). |

Config files: repo `.review/config.toml`, global `~/.config/stet/config.toml` (or XDG equivalent). Precedence: CLI flags > env > repo config > global config > defaults.

## Working directory

The CLI must be run from the repository root (or from a directory under the repo) so that `git rev-parse --show-toplevel` succeeds. Invoke from repo root (e.g. `cd /path/to/repo && stet start --dry-run`).

## Cleanup after interrupted runs

If multiple stet worktrees remain after interrupted runs (e.g. `git worktree list` shows entries under `.review/worktrees/stet-*`), run `stet finish` to remove the current session's worktree, then remove any remaining paths with `git worktree remove <path>`. Phase 6 will add an optional `stet cleanup` command for orphan worktrees.

## State storage and history (history.jsonl)

State lives under `.review/` (session, config, lock). When implemented (Phase 4.5), the CLI will append to `.review/history.jsonl` on user feedback (e.g. on dismiss and/or on finish with findings). Each line is one JSON object with:

- **`diff_ref`**: Ref or SHA for the diff scope.
- **`review_output`**: Array of finding objects (same shape as stdout findings).
- **`user_action`**: Object with:
  - **`dismissed_ids`** (array of strings): Finding IDs the user dismissed.
  - **`dismissals`** (optional): Array of `{ "finding_id": "...", "reason": "..." }` for per-finding reasons. Allowed **`reason`** values: `false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`.
  - **`finished_at`** (optional): When the session was finished (e.g. ISO8601).

Bounded size or rotation (e.g. last N sessions) is applied to avoid unbounded growth. The schema is suitable for future export/upload for org-wide aggregation. Canonical types: `cli/internal/history/schema.go`.

## Review quality and actionability

A finding is **actionable** when the reported issue is real (not already fixed or by design), the suggestion is correct and safe, and the change is within project scope. The default system prompt instructs the model to report only actionable issues and to prefer fewer, high-confidence findings. For the full definition, prompt guidelines, and optional lessons (e.g. common false positives), see [docs/review-quality.md](review-quality.md).

## Environment and pipelines

For pipelines or multiple commands (e.g. `stet doctor ; stet start`), `STET_OLLAMA_BASE_URL`, `STET_TEMPERATURE`, `STET_NUM_CTX`, and other `STET_*` variables must be **exported** (or set in the shell before both commands) so every `stet` invocation sees the same config. Command-prefixed env (e.g. `VAR=value cmd1 ; cmd2`) only applies to the first command; the second process will not see that variable and may fall back to defaults (e.g. `http://localhost:11434`), which can cause "Ollama unreachable" even when the first command succeeded.

## Usage (extension)

1. Spawn the CLI with the desired subcommand and args (e.g. `stet start --dry-run` or `stet run`), from the repository root.
2. On exit code 0: read stdout and parse the single JSON object; use `findings` to populate the panel.
3. On non-zero exit: read stderr for the error message; use exit code 2 to show a specific “Ollama unreachable” message if desired.
