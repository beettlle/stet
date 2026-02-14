# CLI–Extension Contract

This document defines the output and exit-code contract between the Stet CLI and the Cursor extension (Phase 5). The extension spawns the CLI and parses its output.

## Commands that emit findings

- **`stet start [ref]`** — On success, writes findings to stdout (format depends on `--output`).
- **`stet run`** — On success, writes findings to stdout (format depends on `--output`).
- The **`--dry-run`** flag skips the LLM and emits deterministic findings for CI.
- The **`--nitpicky`** flag enables convention- and typo-aware review: the system prompt is augmented to report style, typos, and grammar, and the FP kill list is not applied. Can be set in config (`nitpicky = true`) or env (`STET_NITPICKY=1`). When set on `stet start`, the value is persisted so `stet run` uses it unless overridden.

**Output and progress:**

- **Default:** Progress (worktree path, partition summary, per-hunk lines) is printed to **stderr**. Stdout is **human-readable** (one line per finding: `file:line  severity  message`, then a summary line).
- **Machine output:** Use **`--output=json`** or **`--json`** for machine-parseable JSON on stdout. When **`--json`** or **`--stream`** is used, progress on stderr is suppressed automatically (so **`--quiet`** is optional). Use **`--quiet`** explicitly to suppress progress when using human-readable output. Example: `stet start --dry-run --json` (no need for `--quiet`).
- **Streaming:** Use **`--stream`** together with **`--output=json`** or **`--json`** to receive NDJSON events (one JSON object per line) so the extension can show progress and findings incrementally. **`--stream`** requires JSON output; without `--json` the CLI returns an error. Progress on stderr is suppressed when streaming.

## stdout

**With `--output=json` or `--json`** (required for the extension and any script that parses findings):

**Without `--stream`:** On success (exit code 0), the CLI writes exactly one JSON object, followed by a newline:

```json
{"findings": [ ... ]}
```

- **`findings`**: Array of finding objects. May be empty (e.g. nothing to review).
- Each element has the following fields (see `cli/internal/findings/finding.go` for the canonical schema):
  - **`id`** (string, optional): Stable identifier for the finding. In JSON output the id is always the full stable identifier. In human-readable output (e.g. `stet list`, `stet status --ids`) the CLI shows an abbreviated form (e.g. first 7 characters). Commands that take a finding id (e.g. `stet dismiss`) accept either the full id or a unique prefix of at least 4 characters.
  - **`file`** (string): Relative file path.
  - **`line`** (number, optional): Line number.
  - **`range`** (object, optional): `{"start": n, "end": m}` for multi-line span.
  - **`severity`** (string): `"error"`, `"warning"`, `"info"`, or `"nitpick"`.
  - **`category`** (string): Canonical set for Defect-Focused pipeline and extension: `"security"`, `"correctness"`, `"performance"`, `"maintainability"`, `"best_practice"`. Existing values (`"bug"`, `"style"`, `"testing"`, `"documentation"`, `"design"`) are retained for backward compatibility.
  - **`confidence`** (number): Float 0.0–1.0; model’s certainty. CLI always emits this (default 1.0 when omitted from model output).
  - **`message`** (string): Description of the finding.
  - **`suggestion`** (string, optional): Suggested fix.
  - **`cursor_uri`** (string, optional): Deep link (e.g. `file://` or `cursor://`). When the CLI sets it (when the model omits it), it uses `file://` with absolute path and line (or range) so the extension can open at location.

**With `--stream`** (and `--output=json`/`--json`): On success, the CLI writes **NDJSON** to stdout: one JSON object per line. Each object has a **`type`** field. No final `{"findings": [...]}` is written when streaming.

| `type`    | Other fields | Description |
|-----------|--------------|-------------|
| `progress` | `msg` (string) | Progress message (e.g. "N hunks to review", "Reviewing hunk 1/3: path"). |
| `finding`  | `data` (object) | One finding; same shape as an element of the `findings` array above. |
| `done`     | (none)        | End of stream; no more events. |

Example stream (abbreviated):

```json
{"type":"progress","msg":"2 hunks to review"}
{"type":"progress","msg":"Reviewing hunk 1/2: cli/main.go"}
{"type":"finding","data":{"id":"...","file":"cli/main.go","line":10,"severity":"info","category":"maintainability","confidence":1.0,"message":"..."}}
{"type":"progress","msg":"Reviewing hunk 2/2: pkg/foo.go"}
{"type":"done"}
```

Without `--output=json`, stdout is human-readable (one line per finding plus a summary); format may change. Do not parse it programmatically.

## stderr

Human-readable error and diagnostic messages. When **not** using `--quiet`, progress (worktree path, hunk counts, per-hunk "Reviewing hunk N/M") is also written to stderr. The extension should surface these when the process exits with a non-zero code. For some conditions (e.g. uncommitted changes, or an existing review session), the CLI also prints a one-line **recovery hint** (e.g. `Hint: Run 'stet finish'...`) so the extension or user can suggest the next command.

### Recovery hints

On `stet start` failure, the CLI may print one of the following hints to stderr before the error line:

| Condition | Hint |
|-----------|------|
| Uncommitted changes | `Hint: Commit or stash your changes, then run 'stet start' again.` |
| Worktree already exists | `Hint: Run 'stet finish' to end the current review and remove the worktree, then run 'stet start' again.` |

## Exit codes

| Code | Meaning |
|------|--------|
| **0** | Success; with `--output=json`/`--json`, stdout contains the findings JSON (or NDJSON when `--stream`). |
| **1** | Usage error or other failure (e.g. not a git repo, no session, model not found). |
| **2** | Ollama unreachable (server not running or not reachable). |

## Other commands

- **`stet status`** — Reports baseline, last_reviewed_at, worktree path, finding count, and dismissed count. When the session has them (set at `stet start`), also reports strictness, rag_symbol_max_definitions, and rag_symbol_max_tokens. Exits 1 with "No active session" if no session. Use `--ids` or `-i` to list active finding IDs (ID, file:line, severity, message) for use with `stet dismiss`.
- **`stet list`** — Lists active findings with IDs (same format as `status --ids`). Exits 1 if no active session. Use to copy IDs for `stet dismiss`.
- **`stet dismiss <id> [reason]`** — Adds the finding ID to the session’s dismissed list so it does not resurface in findings output. Optional **reason** (one of `false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`) is recorded for the optimizer. For when to use each reason, see [review-quality.md](review-quality.md#choosing-a-dismissal-reason). Idempotent. Exits 1 if no active session; exits 1 if reason is provided and invalid. Findings can also be **auto-dismissed** when a re-review of the same code (e.g. after the user fixes issues) no longer reports them, so the list shrinks as issues are fixed.
- **`stet finish`** — Ends the session and removes the worktree. Exits 1 if no active session.
- **`stet cleanup`** — Removes orphan stet worktrees (worktrees named `stet-*` that are not the current session’s worktree). Optional; exits 0 when there are no orphans. Exits 1 on error (e.g. not a git repo or `git worktree remove` failure).

## Optimizer (stet optimize)

The optional **`stet optimize`** command invokes an external script (e.g. a Python DSPy optimizer) to improve the system prompt from user feedback. The Go CLI has **no Python or DSPy dependency**; it only runs the configured command. To run the optimizer you need a Python environment with DSPy (or whatever your script requires); the CLI does not install or depend on Python.

- **When to run**: e.g. weekly or after enough feedback has been collected in `.review/history.jsonl`.
- **Input**: The script reads `.review/history.jsonl` (see State storage and history below). The CLI passes the state directory via the **`STET_STATE_DIR`** environment variable when invoking the script.
- **Output**: The script should write `.review/system_prompt_optimized.txt`. When that file exists, the CLI uses it as the system prompt for review (see Phase 3.3).
- **Configuration**: Set the command to run via **`STET_OPTIMIZER_SCRIPT`** or **`optimizer_script`** in repo/global config (e.g. `python3 scripts/optimize.py` or a path to your script). If unset, `stet optimize` exits 1 with a message asking you to configure it.
- **Exit codes**: 0 = success; non-zero = failure (script missing, Python/DSPy error, invalid history, etc.). The CLI propagates the script’s exit code when in 0–255.

For optimizing toward **actionable findings**, see [Review quality and actionability](#review-quality-and-actionability) and [docs/review-quality.md](review-quality.md).

## Configuration

**Precedence:** CLI flags > environment variables > repo config (`.review/config.toml`) > global config (`~/.config/stet/config.toml` or XDG equivalent) > defaults. Canonical defaults and types: `cli/internal/config/config.go`.

### Config schema (full)

| Key / env | Default | Description |
|-----------|---------|-------------|
| `model` / `STET_MODEL` | `qwen3-coder:30b` | Ollama model name. |
| `ollama_base_url` / `STET_OLLAMA_BASE_URL` | `http://localhost:11434` | Ollama API base URL. |
| `context_limit` / `STET_CONTEXT_LIMIT` | 32768 | Token context limit for prompts. |
| `warn_threshold` / `STET_WARN_THRESHOLD` | 0.9 | Warn when estimated tokens exceed this fraction of context limit. |
| `timeout` / `STET_TIMEOUT` | 5m | Timeout for Ollama generate requests (Go duration or seconds). |
| `state_dir` / `STET_STATE_DIR` | (empty → `.review` in repo) | Directory for session, lock, history, optimized prompt. |
| `worktree_root` / `STET_WORKTREE_ROOT` | (empty → `repo/.review/worktrees`) | Directory for stet worktrees. |
| `temperature` / `STET_TEMPERATURE` | 0.2 | Sampling temperature (0–2). Passed to Ollama. |
| `num_ctx` / `STET_NUM_CTX` | 32768 | Model context window size (tokens). Passed to Ollama; 0 = use model default. |
| `optimizer_script` / `STET_OPTIMIZER_SCRIPT` | (none) | Command for `stet optimize` (e.g. `python3 scripts/optimize.py`). |
| `rag_symbol_max_definitions` / `STET_RAG_SYMBOL_MAX_DEFINITIONS` | 10 | Max symbol definitions to inject (0 = disable). |
| `rag_symbol_max_tokens` / `STET_RAG_SYMBOL_MAX_TOKENS` | 0 | Max tokens for symbol-definitions block (0 = no cap). |
| `strictness` / `STET_STRICTNESS` | `default` | Review strictness preset: `strict`, `default`, `lenient`, or `strict+`, `default+`, `lenient+`. Controls confidence thresholds (strict = 0.6/0.7, default = 0.8/0.9, lenient = 0.9/0.95) and whether the false-positive kill list is applied. The "+" presets use the same thresholds but do not apply the FP kill list (more findings shown). |

The + presets (strict+, default+, lenient+) show more findings by not filtering messages that match the built-in FP kill list.

RAG symbol options can also be set via **`--rag-symbol-max-definitions`** and **`--rag-symbol-max-tokens`** on `stet start` and `stet run`; when set, they override config and env. Strictness can also be set via **`--strictness`** on `stet start` and `stet run`; when set, it overrides config and env.

Strictness and RAG symbol options set on **`stet start`** are stored in the session. **`stet run`** uses those stored values when the corresponding flag is **not** set. Explicit flags on **`stet run`** override for that run only; the next run without flags again uses the session values from start.

## Working directory

The CLI must be run from the repository root (or from a directory under the repo) so that `git rev-parse --show-toplevel` succeeds. Invoke from repo root (e.g. `cd /path/to/repo && stet start --dry-run`).

## Cleanup after interrupted runs

If multiple stet worktrees remain after interrupted runs (e.g. `git worktree list` shows entries under `.review/worktrees/stet-*`), run **`stet cleanup`** to remove orphan stet worktrees. Alternatively, run `stet finish` to remove the current session’s worktree, then remove any remaining paths with `git worktree remove <path>`.

## State storage

State lives under `.review/` (or the path given by `state_dir`). Artifacts:

- **`session.json`** — Session state (baseline ref, last_reviewed_at, findings, dismissed_ids, prompt_shadows, and optionally strictness and RAG symbol options from `stet start`).
- **`lock`** — Advisory lock for a single active session.
- **`config.toml`** — Repo-level config (optional).
- **`history.jsonl`** — Feedback log for the optimizer and prompt shadowing (see below).
- **`system_prompt_optimized.txt`** — Written by `stet optimize`; used as system prompt when present.
- **`worktrees/`** — Directory for stet worktrees (default `repo/.review/worktrees` or `worktree_root`). Each entry is `stet-<short-sha>`.

The `.review/` directory is in `.gitignore` by default so state does not pollute version control; it can be removed from `.gitignore` if the team wants to commit state.

## State storage and history (history.jsonl)

Session state (`.review/session.json`) includes **`prompt_shadows`**: on dismiss, the CLI stores `{ "finding_id": "...", "prompt_context": "..." }` for each dismissed finding so it can be used as a negative few-shot in future prompts. The internal **`finding_prompt_context`** map (finding ID → hunk content) is populated during review and used when the user dismisses to record the code context that produced the finding.

The CLI appends to `.review/history.jsonl` on user feedback (on dismiss via `stet dismiss`, on auto-dismiss when re-review no longer reports a finding at that location, and on finish when there are findings). Each line is one JSON object with:

- **`diff_ref`**: Ref or SHA for the reviewed scope (e.g. the HEAD at last review run, i.e. `last_reviewed_at`).
- **`review_output`**: Array of finding objects (same shape as stdout findings).
- **`user_action`**: Object with:
  - **`dismissed_ids`** (array of strings): Finding IDs the user dismissed.
  - **`dismissals`** (optional): Array of `{ "finding_id": "...", "reason": "..." }` for per-finding reasons. Allowed **`reason`** values: `false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`.
  - **`finished_at`** (optional): When the session was finished (e.g. ISO8601).

Rotation keeps the last N records (default 1000) to avoid unbounded growth. The schema is suitable for future export/upload for org-wide aggregation. Canonical types: `cli/internal/history/schema.go`.

## Git note (refs/notes/stet)

On **`stet finish`**, the CLI writes a Git note to **`refs/notes/stet`** at the commit that is current **HEAD**. The note body is a single JSON object with:

| Field | Type | Description |
|-------|------|-------------|
| `session_id` | string | Unique id for the review session (from session or generated on finish). |
| `baseline_sha` | string | Full SHA of the baseline ref. |
| `head_sha` | string | Full SHA of HEAD at finish time. |
| `findings_count` | number | Number of findings in the session. |
| `dismissals_count` | number | Number of dismissed finding IDs. |
| `tool_version` | string | Stet CLI version (e.g. `dev` or set at build via `-ldflags`). |
| `finished_at` | string | When the session finished (RFC3339 UTC). |
| `hunks_reviewed` | number | (optional) Number of hunks reviewed. Zero when not available. |
| `lines_added` | number | (optional) Lines added in reviewed diff. Zero when not available. |
| `lines_removed` | number | (optional) Lines removed in reviewed diff. Zero when not available. |
| `chars_added` | number | (optional) Characters added. Zero when not available. |
| `chars_deleted` | number | (optional) Characters deleted. Zero when not available. |
| `chars_reviewed` | number | (optional) Total characters in hunks reviewed. Zero when not available. |
| `model` | string | (optional) Model name used. Omitted when not available. |
| `prompt_tokens` | number | (optional) Prompt token count. Omitted when `STET_CAPTURE_USAGE=false`. |
| `completion_tokens` | number | (optional) Completion token count. Omitted when `STET_CAPTURE_USAGE=false`. |
| `eval_duration_ns` | number | (optional) Evaluation duration in nanoseconds. Omitted when `STET_CAPTURE_USAGE=false`. |

You can push or fetch this ref (e.g. `git push origin refs/notes/stet`, `git fetch origin refs/notes/stet:refs/notes/stet`) for integration with git-ai or impact analytics. If you run `stet finish` again at the same HEAD, the existing note is overwritten.

### Impact reporting

Use `stet stats` to aggregate metrics from notes and history. See implementation plan Phase 9.

## Review quality and actionability

A finding is **actionable** when the reported issue is real (not already fixed or by design), the suggestion is correct and safe, and the change is within project scope. The default system prompt instructs the model to report only actionable issues and to prefer fewer, high-confidence findings. For the full definition, prompt guidelines, and optional lessons (e.g. common false positives), see [docs/review-quality.md](review-quality.md).

## Environment and pipelines

For pipelines or multiple commands (e.g. `stet doctor ; stet start`), `STET_OLLAMA_BASE_URL`, `STET_TEMPERATURE`, `STET_NUM_CTX`, and other `STET_*` variables must be **exported** (or set in the shell before both commands) so every `stet` invocation sees the same config. Command-prefixed env (e.g. `VAR=value cmd1 ; cmd2`) only applies to the first command; the second process will not see that variable and may fall back to defaults (e.g. `http://localhost:11434`), which can cause "Ollama unreachable" even when the first command succeeded.

## Usage (extension)

1. Spawn the CLI with **`--quiet --json`** (or `--quiet --output=json`). For incremental panel updates, add **`--stream`** so stdout is NDJSON (one event per line). Example: `stet start --dry-run --quiet --json --stream` or `stet run --quiet --json --stream`, from the repository root.
2. On exit code 0: if **not** streaming, read stdout and parse the single JSON object; use `findings` to populate the panel. If **streaming**, read stdout line-by-line; for each line parse the JSON object, and on `type: "progress"` show progress, on `type: "finding"` append the `data` finding and refresh the panel, on `type: "done"` stop scanning.
3. On non-zero exit: read stderr for the error message; use exit code 2 to show a specific “Ollama unreachable” message if desired.
