# Code Review: marksDAM

**Date:** 2026-05-15
**Reviewer:** Automated code audit
**Scope:** Full project (`main.go`, `cmd/`, `config/`, `openai/`, `sql/`)

---

## Summary

| Category | Count |
|----------|-------|
| 🔴 Bugs & logic errors | 6 |
| 🔴 Error handling gaps | 4 |
| 🟡 Concurrency concerns | 2 |
| 🟡 Performance concerns | 3 |
| 🟡 Go idiom violations | 5 |
| 🔴 Missing tests | 1 (zero coverage) |

---

## 🔴 Bugs & Logic Errors

### B1. `sql/createDB.go` — `CreateDB` returns `bool` but always returns `true` or calls `log.Fatal`

**File:** `sql/createDB.go:13`  
**Severity:** High

Every error path in `CreateDB` calls `log.Fatal`, which terminates the process. The function therefore never returns `false`. In `cmd/setup.go:31`, the caller does:

```go
if !dbOpen.CreateDB(dbPath) {
    return fmt.Errorf("failed to create database: %s", dbPath)
}
```

This `if` branch is **unreachable dead code**. A library function should return an `error` and let the caller decide how to handle it — never call `log.Fatal`.

**Impact:** The `cmd.Setup` function has dead error-handling code. If the schema DDL fails, the user sees a raw `log.Fatal` message, not the formatted error returned to the CLI.

---

### B2. `cmd/retrieve.go` — `json.Marshal` errors silently swallowed (3 occurrences)

**File:** `cmd/retrieve.go:52,86,108`  
**Severity:** High

Three separate calls discard the error from `json.Marshal`:

```go
batchJSON, _ := json.Marshal(apiBatch)          // line 52 (Retrieve)
batchJSON, _ := json.Marshal(apiBatch)          // line 86 (processCompletedBatch)
responseJSON, _ := json.Marshal(resp)           // line 108 (processCompletedBatch)
```

When marshalling fails (e.g., non-serializable value), the result is `nil`, and `string(nil)` produces `""`. The database column silently receives an empty string instead of JSON.

**Impact:** Silent data loss. If a batch response body contains a type that `encoding/json` cannot marshal, the raw JSON is lost forever with no warning.

---

### B3. `cmd/submit.go` — `json.Marshal` error silently swallowed (1 occurrence)

**File:** `cmd/submit.go:111`  
**Severity:** High

```go
batchJSON, _ := json.Marshal(batch)
```

Same pattern as B2. The `batch.RawJson` column receives an empty string on marshal failure. The batch record then has no diagnostic data for debugging.

---

### B4. `cmd/retrieve.go` — Completed batch with empty `OutputFileID` becomes a terminal retry loop

**File:** `cmd/retrieve.go:76-79`  
**Severity:** High

```go
case "completed":
    err = processCompletedBatch(ctx, queries, client, batch, apiBatch, nowUnix)
    if err != nil {
        fmt.Printf("  Error processing completed batch: %v\n", err)
    }
```

Inside `processCompletedBatch`:

```go
if apiBatch.OutputFileID == "" {
    return fmt.Errorf("no output file ID in completed batch")
}
```

When this returns an error, the caller just prints it and **does not update the batch status**. The batch remains in the `GetActiveBatches` result set. Every subsequent `retrieve` run will encounter the same batch, call `GetBatch`, see `completed`, call `processCompletedBatch`, and fail identically — **forever**.

**Impact:** Resource leak — a permanently broken batch pollutes every `retrieve` run. The chatty error output grows each time. No circuit breaker exists.

---

### B5. `cmd/submit.go` — Orphaned request records when batch creation fails

**File:** `cmd/submit.go:93-115`  
**Severity:** High

Request records are inserted into the database (`queries.InsertRequest`, lines 93-101) **before** the OpenAI batch is created (`client.CreateBatch`, line 114). If `CreateBatch` fails:

1. The request rows exist in the database with `batch_id = NULL`.
2. `queries.AttachRequestsToBatch` is never called.
3. These requests reference images that have already been marked `status = "submitted"` (lines 140-147).
4. The images will never be picked up by `GetPendingImages` again.

**Impact:** Images are permanently stuck in `submitted` status with orphaned request rows. Recovery requires manual database surgery.

---

### B6. `cmd/submit.go` — `fu.GetFileInfo` error silently dropped in `scanForImages`

**File:** `cmd/submit.go:148`  
**Severity:** Medium

```go
fileInfo, err := fu.GetFileInfo(file.Path)
modTime := time.Now()
if err == nil && fileInfo != nil {
    modTime = fileInfo.ModifiedAt
}
```

When `GetFileInfo` fails (permissions error, deleted file, etc.), `time.Now()` is silently substituted with no warning. The user has no idea the modification time is incorrect.

**Impact:** Incorrect `mtime_unix` values in the database with no diagnostic output. Could affect downstream logic that relies on accurate modification timestamps.

---

## 🔴 Error Handling Gaps

### E1. `sql/openDB.go` — `CloseDB` calls `log.Fatal` on close error

**File:** `sql/openDB.go:33-36`  
**Severity:** High

```go
func CloseDB(db *sql.DB) {
    err := db.Close()
    if err != nil {
        log.Fatal(err)
    }
}
```

This function is called via `defer` in `cmd/submit.go`, `cmd/retrieve.go`, and other places. If `db.Close()` returns an error (e.g., pending WAL checkpoint fails to flush), the **entire process crashes** right before `return nil` — after all the application logic has already succeeded.

**Impact:** A user's operation can complete successfully (batch submitted, results retrieved) but the process exits with code 1 anyway, suggesting failure. Deferred cleanup in `main` would not run.

---

### E2. `config/config.go` — Missing `%w` error wrapping

**File:** `config/config.go`  
**Severity:** Medium

Three functions return bare errors without wrapping:

```go
// SetConfigDir (line 49)
return err

// Load (line 75)  
return err

// Save (line 84)
return err
```

These should use `fmt.Errorf("...: %w", err)` so callers can use `errors.Is` / `errors.As` to inspect the underlying cause.

**Impact:** Callers cannot distinguish between different config failure modes (e.g., file not found vs. permission denied vs. TOML parse error).

---

### E3. `cmd/retrieve.go` — `GetBatch` network failure leaves no `LastCheckedAtUnix`

**File:** `cmd/retrieve.go:48`  
**Severity:** Medium

```go
apiBatch, err := client.GetBatch(ctx, batch.OpenaiBatchID)
if err != nil {
    fmt.Printf("  Error fetching batch status: %v\n", err)
    continue
}
```

When `GetBatch` fails (DNS error, timeout, rate limit), the code prints and continues. The `LastCheckedAtUnix` timestamp is **never updated**. There is no record that the batch was attempted, and no circuit breaker for permanently failing batches.

**Impact:** If a batch ID becomes invalid or the API endpoint is permanently unreachable, every `retrieve` run silently wastes time on the same failed batch with no evidence in the database.

---

### E4. `cmd/retrieve.go` — `GetBatch` error message does not include the failing batch ID

**File:** `cmd/retrieve.go:49`  
**Severity:** Low

```go
fmt.Printf("  Error fetching batch status: %v\n", err)
```

When multiple active batches exist, the user cannot tell **which** batch failed. The batch ID (`batch.OpenaiBatchID`) should be included in the error message.

---

## 🟡 Concurrency Concerns

### C1. `config/config.go` — Global mutable state without synchronization

**File:** `config/config.go:21-23`  
**Severity:** Medium (for current single-threaded CLI usage)

```go
var configDir string
var cfg *Config
```

These package-level variables are mutated by `SetConfigDir` and `Load` without any mutex. Currently safe because the CLI runs single-threaded, but:

- Tests using `t.Parallel()` that call `SetConfigDir` will have data races.
- Future concurrent use (e.g., a server mode) would be silently broken.

**Recommendation:** Either add a `sync.RWMutex` or pass a `*Config` struct explicitly instead of using globals.

---

### C2. `main.go` — No SIGINT/SIGTERM signal handling

**File:** `main.go`  
**Severity:** Medium

Long-running operations (OpenAI batch creation, `retrieve` polling) have no graceful shutdown. Ctrl+C abruptly terminates the process during:

- Database writes (WAL checkpoint may be interrupted)
- OpenAI API calls (in-progress HTTP request aborted)

**Impact:** Potential partial database state. SQLite in WAL mode is crash-safe for the database file itself, but in-flight application-level state (e.g., batch upload in progress, request records being inserted) is lost without cleanup.

---

## 🟡 Performance Concerns

### P1. `cmd/submit.go` — Individual INSERTs without a transaction

**File:** `cmd/submit.go:76-88`  
**Severity:** Medium

Each call to `queries.InsertImageFile` is a separate SQLite transaction. For 1000 images, this means 1000 `BEGIN`/`COMMIT` + fsync cycles.

**Impact:** Linear degradation with image count. A directory with thousands of images will take minutes for what should take milliseconds.

**Recommendation:** `db.BeginTx()` → `queries.WithTx(tx)` → bulk insert → `tx.Commit()`.

---

### P2. `cmd/retrieve.go` — Individual UPDATEs in `processCompletedBatch` without a transaction

**File:** `cmd/retrieve.go:95-140`  
**Severity:** Medium

Each response triggers 2-5 individual `UPDATE`/`INSERT` calls, each a separate transaction. For a batch with 500+ responses, this is 1000+ transactions.

**Recommendation:** Wrap response processing in `db.BeginTx()` / `tx.Commit()`.

---

### P3. `cmd/submit.go` — `requestImageIDs` slice not preallocated

**File:** `cmd/submit.go:90`  
**Severity:** Low

```go
var requestImageIDs []int64
```

Appending in a loop with known size `len(pendingImages)` causes multiple slice reallocations. Should be:

```go
requestImageIDs := make([]int64, 0, len(pendingImages))
```

---

## 🟡 Go Idiom Violations

### I1. `sql/createDB.go` — Function returns `bool` instead of `error`

**File:** `sql/createDB.go:13`  
**Severity:** Medium

Go convention: fallible functions return `(result, error)`. Returning `bool` (always `true` or process crash) is unidiomatic and hides the error from callers. The function should be:

```go
func CreateDB(path string) error
```

---

### I2. `sql/openDB.go` — `CloseDB` discards the error

**File:** `sql/openDB.go:33`  
**Severity:** Medium

Besides the `log.Fatal` issue (E1), the function signature `func CloseDB(db *sql.DB)` does not return an error, forcing the only option to be `log.Fatal`. Should return `error` so callers can decide how to handle it.

---

### I3. `main.go` — Redundant blank import of SQLite driver

**File:** `main.go:10`  
**Severity:** Low

```go
_ "github.com/mattn/go-sqlite3"
```

This driver is also imported in `sql/createDB.go` (via the same blank import). Since `main.go` imports `cmd`, which imports `dbOpen`, the driver registration in `createDB.go` is sufficient. The import in `main.go` is redundant (though harmless).

---

### I4. `main.go` — `flag.ExitOnError` makes error-checking code unreachable

**File:** `main.go:20,34,48,62`  
**Severity:** Low

Every subcommand uses `flag.NewFlagSet("...", flag.ExitOnError)`, which causes `Parse` to call `os.Exit(2)` on error before returning. The subsequent `if err := ...; err != nil { log.Fatal(err) }` blocks are unreachable for parse errors.

Either use `flag.ContinueOnError` and handle errors manually, or remove the dead error checks. The current state mixes both patterns and is misleading to readers.

---

### I5. `cmd/submit.go` — Manual URL construction instead of `net/url`

**File:** `cmd/submit.go:194-205`  
**Severity:** Medium

```go
func buildImageURL(webHost, webURLPath, imagePath string) string {
    if !strings.HasSuffix(webHost, "/") { webHost += "/" }
    imagePath = strings.TrimPrefix(imagePath, "./")
    // ...
}
```

Manual string manipulation is fragile:
- Does not handle URL encoding (spaces, special characters in filenames)
- Double slashes possible (`host//path`)
- Does not validate the host URL format

**Recommendation:** Use `net/url.Parse` + `url.JoinPath(...)` or `url.ResolveReference`.

---

## 🔴 Missing Tests

### T1. Zero test coverage

**File:** All packages  
**Severity:** High

No `*_test.go` files exist anywhere in the project. Key areas needing tests:

| Package | What to test |
|---------|-------------|
| `openai/` | `BuildJSONL` encoding, `ParseBatchOutput` decoding (including malformed input), `ExtractContent` edge cases (empty choices, null body, error response), `BuildBatchRequest` structure |
| `cmd/` | `scanForImages` (empty dir, no images, mixed files), `calculateWebURLPath` (cwd inside/outside web root, symlinks), `buildImageURL` encoding, `processCompletedBatch` error paths |
| `config/` | `Load`/`Save` round-trip, `SetConfigDir` with relative/absolute paths, `GetDBLocation` with custom vs default `DBPath`, `DefaultConfig` field completeness |
| `sql/` | `OpenDB` with invalid path, `CloseDB` on already-closed DB, `CreateDB` on read-only directory |

Table-driven tests with `t.Run` are the project convention (per `AGENTS.md`).

---

## Appendix: Affected Files Summary

| File | Issues |
|------|--------|
| `main.go` | C2, I3, I4 |
| `cmd/submit.go` | B3, B5, B6, P1, P3, I5 |
| `cmd/retrieve.go` | B2 (×3), B4, E3, E4, P2 |
| `cmd/setup.go` | (affected by B1 in `sql/`) |
| `config/config.go` | E2, C1 |
| `openai/batch.go` | (no critical issues found) |
| `sql/createDB.go` | B1, I1 |
| `sql/openDB.go` | E1, I2 |
