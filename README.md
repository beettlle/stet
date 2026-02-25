# Stet

Local code review, powered by your machine. No cloud. No API keys. No data leaves your machine. Uses the hardware you already have—no extra cost.

## Why Stet

- **Free:** Runs on your machine; no per-seat or per-request fees.
- **Private:** No code, prompts, or findings sent off your machine.
- **Review-only:** Focused on review, not auto-fix; you decide what to change.
- **Sustainable:** Uses your local hardware; no datacenter API calls.

## About the Name

**Stet** is a Latin word from proofreading meaning "let it stand"—an instruction to keep existing text unchanged. Stet is review-only: it helps you approve or flag code, not rewrite it.

## Prerequisites

- [Git](https://git-scm.com/)
- [Ollama](https://ollama.com) — install and run `ollama serve`
- Suggested model: `ollama pull qwen3-coder:30b`. You can use a different model via `.review/config.toml` or `STET_MODEL` (e.g. `ollama pull qwen2.5-coder:32b` for a lighter option).

## Quick Start

1. Install Ollama and pull the model (see Prerequisites).
2. Install stet (see Installation).
3. From your repo root: `stet doctor` then `stet start`.

## Example workflow

A typical review cycle on a branch with new commits:

1. **Check environment** — Run `stet doctor` to verify Ollama and the model (optional but recommended once).
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
The script downloads the binary from [GitHub Releases](https://github.com/stet/stet/releases) for your OS and architecture, or builds from source if you run it from the repo or set `STET_REPO_URL`.

```bash
curl -sSL https://raw.githubusercontent.com/stet/stet/main/install.sh | bash
```

**Option 2 — Windows (PowerShell)**  
You may need `-ExecutionPolicy Bypass` for the one-liner:

```powershell
irm https://raw.githubusercontent.com/stet/stet/main/install.ps1 | iex
```

**Option 3 — From source**  
Clone the repo and run the install script (which will build and install), or build manually:

```bash
git clone https://github.com/stet/stet.git
cd stet
./install.sh
```

Or build manually and copy the binary to a directory in your PATH:

```bash
make build
cp bin/stet ~/.local/bin/
```

**Option 4 — Go install**

```bash
go install github.com/stet/stet/cli/cmd/stet@latest
```

(Use the repo’s module path once the project is published.)

**PATH**  
The default install directory is `~/.local/bin` (Mac/Linux) or `%USERPROFILE%\.local\bin` (Windows). Ensure it is in your PATH. For example, add to `~/.bashrc` or `~/.zshrc`: `export PATH="$HOME/.local/bin:$PATH"`.

## Commands

| Command | Description |
|---------|-------------|
| `stet doctor` | Verify Ollama, Git, models |
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
- **`--trace`** — Print internal steps to stderr for debugging (`stet start --trace` or `stet run --trace`).
- **`stet benchmark --model MODEL`** — Benchmark a specific model instead of the configured one.
- **`stet benchmark --warmup`** — Run a warmup call before measuring (load model, discard metrics).

## Cursor Extension

Use Stet inside Cursor (or VSCode) to view findings in a panel, jump to locations, copy to chat, and run "Finish review." The extension runs the CLI and displays results. Load the `extension` folder as an extension development workspace or install from a VSIX.

## Documentation

- On `stet finish`, Stet records session metadata in a Git note (`refs/notes/stet`) for impact analytics and integration with tools like [git-ai](https://github.com/git-ai-project/git-ai).
- [Product Requirements Document](docs/PRD.md)
- [Implementation Plan](docs/implementation-plan.md)
- [CLI–Extension Contract](docs/cli-extension-contract.md) — Configuration and tuning (including RAG and strictness).
- [Review Process Internals](docs/review-process-internals.md) — For contributors modifying the CLI.
- [Review Quality](docs/review-quality.md) — Actionable findings and prompt guidance.

## License

MIT. See [LICENSE](LICENSE).
