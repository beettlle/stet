# Stet

Local code review, powered by your machine. No cloud. No API keys. No data leaves your machine. Uses the hardware you already have—no extra cost.

## Why Stet

- **Free:** Runs on your machine; no per-seat or per-request fees.
- **Private:** No code, prompts, or findings sent off your machine.
- **Review-only:** Focused on review, not auto-fix; you decide what to change.

## About the Name

**Stet** is a Latin word from proofreading meaning "let it stand"—an instruction to keep existing text unchanged. Stet is review-only: it helps you approve or flag code, not rewrite it.

## Prerequisites

- [Git](https://git-scm.com/)
- [Ollama](https://ollama.com) — install and run `ollama serve`
- Suggested model: `ollama pull qwen2.5-coder:32b`

## Quick Start

1. Install Ollama and pull the model (see Prerequisites).
2. Install stet (see Installation).
3. From your repo root: `stet doctor` then `stet start`.

## Installation

### Install Ollama

Install [Ollama](https://ollama.com), then start the server and pull the suggested model:

```bash
ollama serve
ollama pull qwen2.5-coder:32b
```

### Install stet

**From the repo (clone first):**

```bash
git clone <repository-url>
cd stet
./install.sh
```

Ensure `~/.local/bin` is in your PATH.

**Build from source:**

```bash
make build
./bin/stet
```

Or copy `bin/stet` to a directory in your PATH.

**With Go:**

```bash
go install ./cli/cmd/stet
```

(Use the full module path when the project is published.)

## Commands

| Command | Description |
|---------|-------------|
| `stet doctor` | Verify Ollama, Git, models |
| `stet start [ref]` | Start review from baseline |
| `stet run` | Re-run incremental review |
| `stet finish` | Persist state, clean up |
| `stet status` | Show session status |

## Cursor Extension

Use Stet inside Cursor (or VSCode) to view findings in a panel, jump to locations, copy to chat, and run "Finish review." The extension runs the CLI and displays results. Load the `extension` folder as an extension development workspace or install from a VSIX.

## Documentation

- [Product Requirements Document](docs/PRD.md)
- [Implementation Plan](docs/implementation-plan.md)
- [CLI–Extension Contract](docs/cli-extension-contract.md)

## License

MIT. See [LICENSE](LICENSE).
