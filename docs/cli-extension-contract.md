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

Human-readable error and diagnostic messages. The extension should surface these when the process exits with a non-zero code.

## Exit codes

| Code | Meaning |
|------|--------|
| **0** | Success; stdout contains the findings JSON. |
| **1** | Usage error or other failure (e.g. not a git repo, no session, model not found). |
| **2** | Ollama unreachable (server not running or not reachable). |

## Usage (extension)

1. Spawn the CLI with the desired subcommand and args (e.g. `stet start --dry-run` or `stet run`).
2. On exit code 0: read stdout and parse the single JSON object; use `findings` to populate the panel.
3. On non-zero exit: read stderr for the error message; use exit code 2 to show a specific “Ollama unreachable” message if desired.
