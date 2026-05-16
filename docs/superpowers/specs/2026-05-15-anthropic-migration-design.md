# Anthropic Migration Design

**Date:** 2026-05-15
**Status:** Approved

## Context

marksDAM uses OpenAI's Batch API and GPT-4.1 for image analysis. The goal is to replace OpenAI entirely with Anthropic, using the Anthropic Message Batches API with `claude-sonnet-4-5`. Cost control is preserved — the Anthropic Batch API offers the same ~50% discount and async submit→retrieve workflow as OpenAI's.

A secondary goal is reliable cross-platform builds: the binary must compile for macOS ARM64 (native) and Linux x86_64 (cross-compiled from Mac). The current SQLite driver (`mattn/go-sqlite3`) uses CGo and blocks simple cross-compilation; this is resolved as part of this change.

---

## 1. New `anthropic/` Package

Delete `openai/batch.go` and the `openai/` package entirely. Create `anthropic/batch.go`.

The Anthropic Batch API does not use a separate file upload step — requests are submitted inline as JSON. The package surface reflects this:

```go
// ImageInput is the input to BuildRequests.
type ImageInput struct {
    CustomID string
    ImageURL string
}

// BatchResult is a single parsed result from a completed batch.
type BatchResult struct {
    CustomID string
    Content  string      // extracted text; empty on error
    Err      error       // non-nil on errored/expired result
    RawJSON  []byte      // full result JSON for storage
}

// Client wraps the Anthropic SDK client.
type Client struct { /* unexported */ }

func NewClient() *Client

// BuildRequests constructs the inline request slice for CreateBatch.
func BuildRequests(images []ImageInput, model, prompt string) []anthropic.BetaMessageBatchNewParamsRequest

// CreateBatch submits the batch and returns the provider batch object.
func (c *Client) CreateBatch(ctx context.Context, requests []anthropic.BetaMessageBatchNewParamsRequest) (*anthropic.BetaMessageBatch, error)

// GetBatch polls status. Terminal state is "ended" (covers all outcomes).
func (c *Client) GetBatch(ctx context.Context, batchID string) (*anthropic.BetaMessageBatch, error)

// DownloadResults streams results for a batch in "ended" state.
// Each result carries Content (on success) or Err (on errored/expired).
func (c *Client) DownloadResults(ctx context.Context, batchID string) ([]BatchResult, error)
```

**No `ExtractContent` function** — content is extracted during streaming inside `DownloadResults`.

**Image input:** URL-based only (matching current behaviour). Anthropic image blocks use `anthropic.NewURLImageSource(imageURL)`. The `detail` parameter (OpenAI-only) is removed.

**Dependencies:** Add `github.com/anthropics/anthropic-sdk-go`. Remove `github.com/openai/openai-go`.

---

## 2. `cmd/submit.go` Changes

Remove the JSONL build and file upload steps. The new flow after scanning images:

1. Call `anthropic.BuildRequests(images, model, prompt)` to build inline requests.
2. Call `client.CreateBatch(ctx, requests)` — single API call, no file ID returned.
3. Write `ai_batch` record with `provider_batch_id` and `status`.
4. Write `ai_request` records and attach them to the batch.
5. Mark images as `submitted`.

Remove the `detail` config fetch — `config.GetDetail()` is deleted.

`InsertBatch` params change: no `input_file_id`, no `endpoint` (see schema section).

---

## 3. `cmd/retrieve.go` Changes

Anthropic batches have two states: `in_progress` and `ended`. Replace the OpenAI multi-status switch with:

```
switch apiBatch.ProcessingStatus {
case "in_progress":
    update last_checked_at_unix, print progress counts
case "ended":
    call processEndedBatch(...)
}
```

`processEndedBatch` calls `client.DownloadResults()` and iterates `[]BatchResult`. No file download. Each result with `Err != nil` marks the image and request as `error`; each successful result stores `RawJSON` in `ai_result` and updates the image with `Content`.

Remove `UpdateBatchFailed` and `UpdateBatchCompleted` calls — replace with a single `UpdateBatchEnded` query.

Remove the import of `sdkOpenai "github.com/openai/openai-go"`.

---

## 4. DB Schema Migration

### New schema (`sql/schema.sql`)

Replace the three `openai_*` tables with:

**`ai_batch`** — file-ID columns dropped, `openai_batch_id` → `provider_batch_id`, status values simplified:

```sql
CREATE TABLE IF NOT EXISTS ai_batch (
    id INTEGER PRIMARY KEY,
    provider_batch_id TEXT UNIQUE NOT NULL,
    status TEXT NOT NULL,                  -- in_progress | ended
    request_count INTEGER,
    submitted_at_unix INTEGER NOT NULL,
    last_checked_at_unix INTEGER,
    completed_at_unix INTEGER,
    raw_json TEXT
);
CREATE INDEX IF NOT EXISTS idx_batch_status ON ai_batch(status);
```

**`ai_request`** — unchanged columns, foreign key updated to `ai_batch`:

```sql
CREATE TABLE IF NOT EXISTS ai_request (
    id INTEGER PRIMARY KEY,
    image_file_id INTEGER NOT NULL REFERENCES image_file(id),
    batch_id INTEGER REFERENCES ai_batch(id),
    custom_id TEXT UNIQUE NOT NULL,
    model TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'queued',  -- queued | submitted | completed | error
    error TEXT,
    created_at_unix INTEGER NOT NULL,
    updated_at_unix INTEGER
);
CREATE INDEX IF NOT EXISTS idx_req_image ON ai_request(image_file_id);
CREATE INDEX IF NOT EXISTS idx_req_batch ON ai_request(batch_id);
CREATE INDEX IF NOT EXISTS idx_req_custom_id ON ai_request(custom_id);
```

**`ai_result`** — unchanged columns, foreign key updated to `ai_request`:

```sql
CREATE TABLE IF NOT EXISTS ai_result (
    id INTEGER PRIMARY KEY,
    request_id INTEGER NOT NULL REFERENCES ai_request(id),
    output_json TEXT NOT NULL,
    created_at_unix INTEGER NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_result_req ON ai_result(request_id);
```

### Migration script (`sql/migrate_v2.sql`)

A standalone SQL script to rename existing tables in-place (create-copy-drop pattern, required for SQLite):

1. Create `ai_batch`, copy from `openai_batch` (mapping `openai_batch_id` → `provider_batch_id`, dropping file-ID columns).
2. Create `ai_request`, copy from `openai_request` with updated foreign key.
3. Create `ai_result`, copy from `openai_result` with updated foreign key.
4. Drop `openai_result`, `openai_request`, `openai_batch` in dependency order.

This script is for users with existing databases. New databases created via `setup` use `schema.sql` directly and never need migration.

### `sql/query.sql` changes

Update all table and column references:
- `openai_batch` → `ai_batch`, `openai_batch_id` → `provider_batch_id`
- `openai_request` → `ai_request`
- `openai_result` → `ai_result`
- Remove `input_file_id`, `output_file_id`, `error_file_id`, `endpoint` from `InsertBatch`
- `GetActiveBatches`: `status NOT IN ('completed',...)` → `status != 'ended'`
- Replace `UpdateBatchCompleted` + `UpdateBatchFailed` with `UpdateBatchEnded`

Run `cd sql && sqlc generate` after updating `.sql` files.

---

## 5. Config Changes

| Setting | Change |
|---|---|
| Env var | `OPENAI_API_KEY` → `ANTHROPIC_API_KEY` |
| Default model | `gpt-4.1` → `claude-sonnet-4-5` |
| `detail` field | Removed from config and `config.GetDetail()` deleted |
| `completionWindow` | Removed (Anthropic batch window is fixed) |

Update `config/config.go`, `cmd/config.go`, `main.go`, and the `setup` command's generated `config.toml` template.

Update `image_file.description` column comment in `schema.sql` (remove "OpenAI response text").

---

## 6. SQLite Driver Swap

`mattn/go-sqlite3` uses CGo and prevents cross-compilation without a Linux CGo toolchain installed on the Mac. Replace with `modernc.org/sqlite`, a pure-Go port with the same `database/sql` interface.

Changes:
- `sql/createDB.go`: change import `_ "github.com/mattn/go-sqlite3"` → `_ "modernc.org/sqlite"`
- `sql/createDB.go` + `sql/openDB.go`: change driver string `"sqlite3"` → `"sqlite"` in all `sql.Open()` calls
- `go.mod`: `go get modernc.org/sqlite`, `go mod tidy` to drop `mattn/go-sqlite3`

DSN format (`file:path?_busy_timeout=5000&_foreign_keys=1`) and pragma execution are compatible between the two drivers.

---

## 7. Build: `justfile`

Add a `justfile` at the repo root with a `build-all` recipe:

```just
# Build for current platform only
build:
    go build -o marksdam .

# Build for all supported platforms
build-all:
    go build -o marksdam-darwin-arm64 .
    GOOS=linux GOARCH=amd64 go build -o marksdam-linux-amd64 .

# Run tests
test:
    go test ./...

# Run tests with race detector
test-race:
    go test -race ./...

# Lint
lint:
    golangci-lint run

# Format
fmt:
    gofmt -w . && goimports -w .

# Regenerate sqlc
sqlc:
    cd sql && sqlc generate
```

---

## 8. Tests

- Delete `openai/batch_test.go`.
- Create `anthropic/batch_test.go` with table-driven tests covering `BuildRequests` (request shape, URL embedding, prompt embedding) and `BatchResult` parsing (succeeded/errored/expired cases).
- Update `cmd/submit_test.go` and `cmd/retrieve_test.go`: remove references to `input_file_id`, `detail`, OpenAI status values; update to Anthropic status model.
- Verify `go test ./...` passes for both packages after changes.

---

## 9. Verification

```bash
# Unit tests
go test ./...
go test -race ./...

# Build both targets (requires just)
just build-all

# Smoke test: setup fresh DB, submit images, retrieve results
export ANTHROPIC_API_KEY=<key>
go run . setup -path /tmp/dam-test
cd /path/to/images && go run . submit -config /tmp/dam-test
go run . retrieve -config /tmp/dam-test
```

Confirm: images reach `status = completed` in the DB, `description` is populated, `ai_result.output_json` contains the raw Anthropic response.
