# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Navigation (CRITICAL)
This project is indexed using `gograph`. **DO NOT use `grep` or `cat` for structural Go code analysis.**

1. Before answering architecture or repository questions, inspect the available `gograph_*` MCP tools for the current project and use them. Each project ships its own gograph MCP server; pick the matching one.
2. If MCP tools are not available, run `gograph build .` in the terminal to ensure the index is fresh, then use the CLI commands (e.g., `gograph implementers <InterfaceName>`).
3. If the codebase is in a compilable state, building with `gograph build . --precise` enables strict type-checked interface analysis and highly precise call edges.
4. To extract a function body or mock stub without reading the whole file, use the source tool.
5. Use `grep` ONLY for string literals, configuration files (.env), or markdown documentation.

## Commands

```bash
go build -o marksdam .          # Build binary for current platform
go run . <command> [options]    # Run without building
go build ./...                  # Build all packages
go test ./...                   # Run all tests
go test -race ./...             # Run tests with race detector
go test ./cmd/... -run TestName # Run a single test
go vet ./...                    # Vet all packages
golangci-lint run               # Lint
gofmt -w . && goimports -w .    # Format
GOOS=linux GOARCH=amd64 go build -o marksdam-linux-amd64 .  # Cross-compile for Linux
```

## Environment

- `OPENAI_API_KEY` must be set for `submit` and `retrieve` commands.
- The binary operates on the **current working directory** when scanning for images (`submit` scans `.`).

## Architecture

This is a CLI tool for batch-submitting images to OpenAI for metadata extraction, storing results in SQLite.

### Data flow

1. **`setup`** — creates `config.toml` and `marksdam.db` in a config directory.
2. **`submit`** — scans CWD recursively for images → inserts into `image_file` table → builds JSONL → uploads to OpenAI Files API → creates an OpenAI Batch job → records everything in DB.
3. **`retrieve`** — polls all non-terminal batches → on completion, downloads the output JSONL → parses each response → stores the raw JSON in `openai_result` and updates `image_file.description` + `image_file.status`.

### Package responsibilities

| Package | Role |
|---|---|
| `main.go` | CLI entry point; flag parsing per subcommand, routes to `cmd.*` |
| `cmd/` | One file per subcommand (`setup`, `submit`, `retrieve`, `config`). All business logic lives here. |
| `config/` | Package-level singletons `configDir` and `cfg`. Call `SetConfigDir` then `Load` before any getter. |
| `openai/` | Thin wrapper around `github.com/openai/openai-go` — JSONL building, file upload, batch CRUD, response parsing. |
| `sql/` | `openDB.go` / `createDB.go` for connection management; `schema.sql` + `query.sql` are the source of truth; `generated_files/` is produced by `sqlc`. |

### URL construction

`submit` builds public image URLs as: `webHost + relPath(systemWebRoot → CWD) + "/" + imageRelPath`. Images must be publicly accessible for OpenAI to fetch them.

### sqlc-generated code

`sql/generated_files/` is generated from `sql/query.sql` + `sql/schema.sql` via `sqlc generate` (config in `sql/sqlc.yaml`, package name `dbAccess`). Edit the `.sql` files, then regenerate — do not edit generated files directly.

```bash
cd sql && sqlc generate
```

### Image status lifecycle

`new` → `submitted` (on batch creation) → `completed` or `error` (on retrieve).  
Re-running `submit` picks up any images with status `new` or `error`.

## Style & Idioms

- Error wrapping: `fmt.Errorf("doing X: %w", err)`
- `context.Context` as first parameter on functions that call external services
- Use `errors.Is` / `errors.As`, not `==` for error comparison
- Table-driven tests with `t.Run`; no testify
- Doc comments on every exported name, starting with the name
