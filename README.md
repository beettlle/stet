# Stet

Local code review, powered by your machine. **By default** Stet talks to [Ollama](https://ollama.com) on your computer: no cloud API keys required for that path, and prompts stay on your machine. Uses the hardware you already have—no extra cost.

## Why Stet

- **Free:** Default setup runs locally; no per-seat or per-request fees to the vendor for typical Ollama use.
- **Private (default path):** With Ollama on localhost, code and prompts are not sent to a remote SaaS.
- **Review-only:** Focused on review, not auto-fix; you decide what to change.
- **Sustainable:** Default setup uses your local hardware instead of datacenter APIs.

## Performance

Measured October 2026 on a Mac Studio (Apple M1 Ultra, 64 GB) with `qwen/qwen3.8-27b` served by LM Studio:

| | |
|---|---|
| Throughput | ~21 tokens/s end to end (`stet benchmark --warmup`) |
| API cost per review | $0 on a local model (the same tokens cost ~$0.10–$0.20 per review at Claude Sonnet 5.5 / Opus 5.5 list prices) |
| Seeded-bug test | 10 of 12 planted bugs caught in all 3 runs, including SQL injection and path traversal |
| Test coverage | 83.9%, enforced in CI (77% project, 72% per file) |

Speed depends on your hardware and model; run `stet benchmark` to measure yours.

## About the Name

**Stet** is a Latin word from proofreading meaning "let it stand"—an instruction to keep existing text unchanged. Stet is review-only: it helps you approve or flag code, not rewrite it.

## Prerequisites

- [Git](https://git-scm.com/)
- **Default LLM — Ollama:** Install [Ollama](https://ollama.com), run `ollama serve`, and pull a model (e.g. `ollama pull qwen3-coder:30b`). Override the model in `.review/config.toml` or with `STET_MODEL` (e.g. `qwen2.5-coder:32b`).
- **Optional — OpenAI-compatible HTTP API:** See [OpenAI-compatible API (optional)](#openai-compatible-api-optional) below; no Ollama install required for that mode.

## Quick Start

1. Set up your LLM (Ollama by default; see Prerequisites).
2. Install stet (see Installation).
3. From your repo root: `stet doctor` then `stet start`.

`stet doctor` checks Git and reachability of your **configured** LLM (Ollama or OpenAI-compat base URL). On success it may still print `Ollama OK` even when `provider = "openai"`; use exit code **0** and the `Model:` line as the signal.

## Example workflow

A typical review cycle on a branch with new commits:

1. **Check environment** — Run `stet doctor` to verify Git and the configured LLM endpoint (optional but recommended once).
2. **Start the review** — Run `stet start` or `stet start HEAD~3` to review the last 3 commits; wait for the run to complete.
3. **Inspect findings** — Use `stet status` or `stet list` to see findings and IDs. In the Cursor extension, use the findings panel and “Copy for chat.”
4. **Triage** — Run `stet dismiss <id>` or `stet dismiss <id> <reason>` for findings you want to ignore (reasons: `false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope`); fix code as needed. For when to use each reason, see [Choosing a dismissal reason](docs/review-quality.md#choosing-a-dismissal-reason).
5. **Re-review** — Run `stet run` to re-review only changed hunks. Findings that the model no longer reports (e.g. because you fixed the code) are **automatically dismissed**, so the active list shrinks as issues are fixed—no need to manually dismiss each one.
6. **Finish** — When done, run `stet finish` to persist state and remove the review worktree.

You can also run start, run, and finish from the Cursor extension.

## Installation

### Install Ollama

Install [Ollama](https://ollama.com), then start the server and pull the suggested model:

```bash
ollama serve
ollama pull qwen3-coder:30b   # or qwen2.5-coder:32b for a lighter option
```

### Install stet

**Option 1 — Install script (Mac/Linux, recommended)**  
The script downloads the binary for your OS and architecture from [GitHub Releases](https://github.com/beettlle/stet/releases), verifies it against the release's `checksums.txt`, and installs it. If no release binary can be downloaded, it builds from source when run from the repo or when `STET_REPO_URL` is set.

```bash
curl -fsSL https://raw.githubusercontent.com/beettlle/stet/main/install.sh | bash
```

To install a specific version instead of the latest:

```bash
curl -fsSL https://raw.githubusercontent.com/beettlle/stet/main/install.sh | STET_RELEASE_TAG=v1.0.0 bash
```

**Option 2 — Windows (PowerShell)**  
You may need `-ExecutionPolicy Bypass` for the one-liner. Set `$env:STET_RELEASE_TAG = "v1.0.0"` first to pin a version.

```powershell
irm https://raw.githubusercontent.com/beettlle/stet/main/install.ps1 | iex
```

**Option 3 — Manual download**  
Download `stet-<os>-<arch>` and `checksums.txt` from the [Releases page](https://github.com/beettlle/stet/releases), check the hash (`shasum -a 256 -c checksums.txt --ignore-missing`), rename the file to `stet`, `chmod +x` it, and move it into a directory in your PATH. On macOS, a binary downloaded through a browser is quarantined by Gatekeeper; clear it with `xattr -d com.apple.quarantine stet`. (The install script uses `curl`, which does not set the quarantine flag.)

**Option 4 — From source**  
Clone the repo and run the install script (which will build and install), or build manually:

```bash
git clone https://github.com/beettlle/stet.git
cd stet
./install.sh
```

Or build manually and copy the binary to a directory in your PATH:

```bash
make build
cp bin/stet ~/.local/bin/
```

**PATH**  
The default install directory is `~/.local/bin` (Mac/Linux) or `%USERPROFILE%\.local\bin` (Windows). Ensure it is in your PATH. For example, add to `~/.bashrc` or `~/.zshrc`: `export PATH="$HOME/.local/bin:$PATH"`.

## OpenAI-compatible API (optional)

Stet can use any server that exposes an **OpenAI-compatible** chat/completions HTTP API (for example **LM Studio**’s local server, or other gateways). Configure it in `.review/config.toml` (or your global Stet config) or with environment variables:

| Setting | TOML key | Environment variable | Notes |
|--------|----------|----------------------|--------|
| Provider | `provider = "openai"` | `STET_PROVIDER=openai` | Default is `ollama`. |
| Base URL | `openai_base_url = "http://127.0.0.1:1234/v1"` | `STET_OPENAI_BASE_URL` | Include the `/v1` prefix if your server uses it (LM Studio default is often port `1234`). |
| Completion cap | `max_completion_tokens = 4096` | `STET_MAX_COMPLETION_TOKENS` | Sent as OpenAI **`max_tokens`** (how many tokens the model may *generate*). **Default 4096.** This is **separate** from the context window (`num_ctx`, `--context`, `STET_NUM_CTX`), which sizes the prompt; large `--context` does not automatically raise the completion cap. |

**Privacy and keys:** Pointing at **localhost** keeps traffic on your machine (subject to that server’s behavior). Pointing at a **remote** vendor URL means prompts may leave your machine and you may need API keys as required by that server—Stet does not change those rules.

Full precedence and every key are documented in the [CLI–Extension Contract](docs/cli-extension-contract.md#configuration).

## Commands

| Command | Description |
|---------|-------------|
| `stet doctor` | Verify Git and configured LLM reachability (Ollama or OpenAI-compat) |
| `stet skill` | Print Agent Skill Markdown for LLM integration (e.g. save as SKILL.md in `.claude/skills/stet-integration/`) |
| `stet benchmark` | Measure model throughput (tokens/s) for the configured model |
| `stet commitmsg` | Generate a conventional git commit message from uncommitted changes (local LLM); `--commit` to commit with it, `--commit-and-review` to commit then run review |
| `stet start [ref]` | Start review from baseline |
| `stet run` | Re-run incremental review |
| `stet rerun` | Re-run full review (all hunks) with same or overridden parameters; use `--replace` to overwrite previous findings; requires an active session |
| `stet finish` | Persist state, clean up; writes session note to `refs/notes/stet` for impact analytics |
| `stet status` | Show session status |
| `stet list` | List active findings with IDs (for use with dismiss) |
| `stet dismiss <id> [reason]` | Mark a finding as dismissed; optional reason: `false_positive`, `already_correct`, `wrong_suggestion`, `out_of_scope` |
| `stet cleanup` | Remove orphan stet worktrees |
| `stet optimize` | Run optional DSPy optimizer (history → optimized prompt) |
| `stet stats [volume\|quality\|energy]` | Aggregate impact metrics from notes and history |
| `stet --version` | Print installed version |

## Useful flags

- **`--nitpicky`** — Report style, typos, and grammar (config: `nitpicky = true` or `STET_NITPICKY=1`).
- **`--verify`** — Run the critic (second-pass verification) on each finding and drop the ones it rejects; add `--critic-backend systemone` to use a System One decision server (one ~100 ms request per hunk) instead of a second LLM pass. Critic keys (including env-only `STET_SYSTEMONE_API_KEY`) are documented in the [CLI–Extension Contract](docs/cli-extension-contract.md#configuration).
- **`--trace`** — Print internal steps to stderr for debugging (`stet start --trace` or `stet run --trace`).
- **`--rules-max-tokens`** — Token budget for Cursor rule review digests in the prompt (config: `rules_max_tokens` or `STET_RULES_MAX_TOKENS`; default `0` = 15% of `context_limit`, clamped to 1000–8192). See [Cursor rules in the review prompt](docs/cli-extension-contract.md#cursor-rules-in-the-review-prompt).
- **`--timeout`** — Per-request timeout for long or large-context reviews (e.g. `stet start --timeout 45m`). See [CLI–Extension Contract](docs/cli-extension-contract.md#long-reviews-and-large-context).
- **`stet benchmark --model MODEL`** — Benchmark a specific model instead of the configured one.
- **`stet benchmark --warmup`** — Run a warmup call before measuring (load model, discard metrics).

## Cursor Extension

Use Stet inside Cursor (or VSCode) to view findings in a panel, jump to locations, copy to chat, and run "Finish review." The extension runs the CLI and displays results. Load the `extension` folder as an extension development workspace or install from a VSIX.

By default, **Start review** runs a production review (`stet start` or incremental `stet run` when a session is active) with JSON streaming. Set **`stet.dryRun`** to `true` in workspace or user settings to use `--dry-run` instead (no LLM; useful for development or CI).

Filter what appears in the findings panel with **`stet.minConfidence`** (0–1; findings below the threshold are hidden) and **`stet.categories`** (optional allowlist; empty shows all). Filtering is display-only and does not change the CLI session.

## Documentation

- On `stet finish`, Stet records session metadata in a Git note (`refs/notes/stet`) for impact analytics and integration with tools like [git-ai](https://github.com/git-ai-project/git-ai).
- [Product Requirements Document](docs/PRD.md)
- [Implementation Plan](docs/implementation-plan.md)
- [CLI–Extension Contract](docs/cli-extension-contract.md) — Configuration and tuning (including RAG and strictness).
- [Review Process Internals](docs/review-process-internals.md) — For contributors modifying the CLI.
- [Review Quality](docs/review-quality.md) — Actionable findings and prompt guidance.

## License

MIT. See [LICENSE](LICENSE).
