# Contract: Optimizer Sidecar

**Version**: 1.0.0  
**Phase**: 0  
**Consumers**: Go `stet optimize` (`cli/cmd/stet/main.go`)

---

## Invocation

Go executes the configured optimizer script with environment variables set. No stdin protocol in V0.

| Env var | Required | Description |
|---------|----------|-------------|
| `STET_STATE_DIR` | yes | Directory containing `history.jsonl` (typically `.review/`) |
| `STET_OPTIMIZER_SCRIPT` | set by Go | Full command (e.g. `python3 scripts/optimize.py`) — script receives env only |

Config alternative: `optimizer_script` in `.review/config.toml` (same command string).

---

## Inputs

| Path | Required | Semantics |
|------|----------|-----------|
| `$STET_STATE_DIR/history.jsonl` | yes* | JSONL history records; includes rotated `.gz` archives |
| `$STET_STATE_DIR/history.jsonl.*.gz` | optional | Rotated archives per Go ReadRecords |

\*Missing or unreadable → **exit non-zero**. Valid file with zero dismissal records → **exit 0**, no output file (see Exit codes).

---

## Outputs

| Path | Condition | Semantics |
|------|-----------|-----------|
| `$STET_STATE_DIR/system_prompt_optimized.txt` | dismissals exist | Full system prompt (default + lessons); UTF-8 text |
| stderr | always | Summary stats: record count, dismissal breakdown |

---

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success: prompt written OR empty-history no-op |
| 1 | Missing/unreadable history |
| 2 | Validation failure (prompt contract violated) |
| other | Propagated from Python unhandled errors |

Go propagates exit code to `stet optimize` caller.

---

## Behavioral contract

1. Read history via `history_loader` (mirror Go semantics).
2. If zero dismissals → print stderr message, exit 0, **do not write** output file.
3. Aggregate dismissals by reason, category, normalized message.
4. Build prompt: `DefaultSystemPrompt` + `## Lessons learned` section.
5. Enforce 2× default length cap; truncate by dismissal frequency if over.
6. Run `prompt_validator`; reject write on failure.
7. Write atomically (write temp + rename preferred).
8. Deterministic: identical history → identical output.

---

## Lessons mapping

| Dismiss reason | Prompt guidance |
|----------------|-----------------|
| `false_positive`, `wrong_suggestion` | Negative examples (use `prompt_context` when present) |
| `already_correct` | Verify-before-reporting reinforcement |
| `out_of_scope` | Scope-filter guidance |

---

## Non-goals (V0)

- No DSPy / LLM calls inside optimizer
- No network I/O
- No modification of `history.jsonl`
