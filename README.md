# Mark's DAM

A command-line Digital Asset Management tool that recursively scans directories for images, submits them to Anthropic's Message Batches API for analysis, and stores the results in a local SQLite database. Each image gets a description, OCR text, and searchable tags extracted by `claude-sonnet-4-5`.

---

## Technical Overview

### Architecture

marksDAM is a single Go binary with four subcommands (`setup`, `submit`, `retrieve`, `config`). State is persisted in a SQLite database managed via [sqlc](https://sqlc.dev)-generated queries. All AI requests go through the Anthropic Message Batches API — an async, ~50% cheaper alternative to synchronous API calls.

### Data flow

1. **`setup`** creates `config.toml` and `marksdam.db` in a config directory.
2. **`submit`** scans the current working directory recursively, upserts image records into `image_file`, builds inline batch requests, and submits them to Anthropic in a single API call. The returned batch ID and each per-image request are recorded in the database, and images are marked `submitted`.
3. **`retrieve`** polls all batches where `status != 'ended'`. When Anthropic reports `ended`, results are streamed and written to `ai_result`; `image_file.description` and `image_file.status` are updated accordingly. Images with failed or expired requests are marked `error` and picked up by the next `submit` run.

### Package responsibilities

| Package | Role |
|---|---|
| `main.go` | CLI entry point; flag parsing per subcommand, routes to `cmd.*` |
| `cmd/` | One file per subcommand. All business logic lives here. |
| `config/` | TOML config loading. Call `SetConfigDir` then `Load` before any getter. |
| `anthropic/` | Thin wrapper around `github.com/anthropics/anthropic-sdk-go` — batch request building, batch CRUD, result streaming. |
| `sql/` | Schema, queries, and connection management. `generated_files/` is produced by `sqlc generate` — do not edit directly. |

### Database schema

| Table | Purpose |
|---|---|
| `image_file` | One row per discovered image; tracks URL, status, and extracted description |
| `image_meta` | Unique tag/keyword strings |
| `meta_map` | Many-to-many join between `image_file` and `image_meta` |
| `ai_batch` | One row per Anthropic batch submission |
| `ai_request` | One row per image per batch |
| `ai_result` | Raw Anthropic response JSON, one row per completed request |

**Image status lifecycle:** `new` → `submitted` → `completed` or `error`. Re-running `submit` picks up any image with status `new` or `error`.

### URL construction

Images must be publicly accessible for Anthropic to fetch them. marksDAM builds URLs as:

```
{webHost} + relPath({systemWebRoot} → CWD) + "/" + {imageRelPath}
```

For example, with `webHost = "https://example.com/"`, `systemWebRoot = "/var/www"`, and CWD `/var/www/photos/2025`, an image `cats/tabby.jpg` becomes `https://example.com/photos/2025/cats/tabby.jpg`.

### Build

The project uses `modernc.org/sqlite` (pure Go — no CGo), so cross-compilation works without a C toolchain.

```bash
# Current platform
just build

# macOS ARM64 + Linux x86_64
just build-all

# Tests
just test
just test-race
```

Or without `just`:

```bash
go build -o marksdam .
GOOS=linux GOARCH=amd64 go build -o marksdam-linux-amd64 .
go test ./...
```

### Dependencies

- `github.com/anthropics/anthropic-sdk-go` — Anthropic Go SDK
- `modernc.org/sqlite` — pure-Go SQLite driver
- `github.com/BurntSushi/toml` — TOML parser
- `github.com/dxau-dev/fileUtilities` — recursive directory scanning
- `github.com/dxau-dev/dateUtilities` — UTC date helpers

---

## Usage

### Prerequisites

- Go 1.22+
- An Anthropic API key

```bash
export ANTHROPIC_API_KEY="your-api-key-here"
```

Images must be served at a publicly accessible URL. Anthropic fetches each image directly during batch processing.

### 1. Initialise

Create a config directory containing `config.toml` and `marksdam.db`:

```bash
marksdam setup -path ./dam-config
```

### 2. Configure

Edit `dam-config/config.toml`:

```toml
webHost         = "https://example.com/"
systemWebRoot   = "/var/www"
imageExtensions = ["gif", "jpeg", "jpg", "png", "webp"]
dbPath          = "marksdam.db"
model           = "claude-sonnet-4-5"
prompt          = "..."
```

| Option | Description | Default |
|---|---|---|
| `webHost` | Base URL where images are publicly accessible | `https://example.com/` |
| `systemWebRoot` | Filesystem path to web server root, used to calculate the URL path | `/var/www` |
| `imageExtensions` | Extensions to scan for | `["gif","jpeg","jpg","png","webp"]` |
| `dbPath` | Database filename, relative to config directory | `marksdam.db` |
| `model` | Anthropic model | `claude-sonnet-4-5` |
| `prompt` | Prompt sent with every image | See default in `config/config.go` |

### 3. Submit images

Navigate to the directory you want to process, then run `submit`:

```bash
cd /var/www/photos/2025
marksdam submit -config /path/to/dam-config
```

`submit` scans the current directory recursively, skipping images already in the database with status `submitted` or `completed`. It prints the resulting Anthropic batch ID.

### 4. Retrieve results

```bash
marksdam retrieve -config /path/to/dam-config
```

Run this periodically. It checks all active batches and, when Anthropic reports a batch as `ended`, downloads and stores the results. Each image's `description` column is populated with the extracted text.

Typical Anthropic batch turnaround is minutes to hours. The Anthropic dashboard shows progress if you need to monitor before the next `retrieve` run.

### 5. Inspect configuration

```bash
marksdam config -path /path/to/dam-config -print
```

### Full example

```bash
export ANTHROPIC_API_KEY="sk-ant-..."

# One-time setup
marksdam setup -path ~/dam-config

# Edit config to match your web host and image server root
nano ~/dam-config/config.toml

# Submit images from a directory
cd /var/www/photos/2025
marksdam submit -config ~/dam-config

# Poll for results (run again later if still in_progress)
marksdam retrieve -config ~/dam-config

# Submit a second directory using the same config/database
cd /var/www/photos/2024
marksdam submit -config ~/dam-config
marksdam retrieve -config ~/dam-config
```

### Existing databases (migration from OpenAI version)

If you have a database created by the previous OpenAI-based version, run the migration script once:

```bash
sqlite3 /path/to/marksdam.db < sql/migrate_v2.sql
```

This renames the `openai_*` tables to `ai_*` and maps historical status values to the Anthropic model. New databases created by `setup` do not need this step.
