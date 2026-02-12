# Stet

A review-only, local LLM–powered code review tool that uses a read-only git worktree for stable scope, persists approved hunks to avoid flip-flopping, and surfaces findings in a Cursor extension with a single "Finish review" action.

## Build

### CLI (Go)

From the repository root, run:

```bash
make build
```

This produces:

- **`bin/stet`** — native binary (use when running in the dev container or on your host)
- **`bin/stet-linux-amd64`** — Linux amd64 (e.g. dev container, Linux CI)
- **`bin/stet-darwin-amd64`** — Darwin amd64 (Intel Mac)

To build only the native binary: `go build -o bin/stet ./cli/cmd/stet`

### Extension (TypeScript)

From the repository root:

```bash
cd extension
npm install
npm run compile
```

## Run

### CLI

After building:

```bash
./bin/stet
```

Or without building:

```bash
go run ./cli/cmd/stet
```

### Extension

Load the extension in Cursor: open the `extension` folder as the extension development workspace, or install from a VSIX. The placeholder command "Stet: Start review" is available from the Command Palette.

## Documentation

- [Product Requirements Document](docs/PRD.md)
- [Implementation Plan](docs/implementation-plan.md)
