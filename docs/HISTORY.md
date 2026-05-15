# History

## 2026-05-15

### Code audit and bug fixes

An automated code audit (`docs/code-review.md`) identified 6 bugs, 4 error handling gaps, 2 concurrency concerns, 3 performance concerns, and 5 Go idiom violations across the codebase. All high-severity bugs and error handling gaps were addressed in this session.

**`sql/createDB.go` — Changed `CreateDB` to return `error` instead of `bool`**
The function called `log.Fatal` on every error path, terminating the process instead of returning a failure to the caller. This made the `if !dbOpen.CreateDB(...)` check in `cmd/setup.go` unreachable dead code and denied the CLI the chance to display a formatted error message.

**`sql/openDB.go` — Fixed variable shadowing bug and made `CloseDB` return `error`**
`OpenDB` contained two `:=` declarations (`err := db.Close()`) that shadowed the outer `err` from the failed PRAGMA/ping call. When `db.Close()` succeeded, the function returned `(nil, nil)` — silently reporting success despite a real failure. `CloseDB` called `log.Fatal` on close error, which could crash the process after all application logic had already succeeded.

**`config/config.go` — Added `%w` error wrapping throughout**
`SetConfigDir`, `Load`, and `Save` returned bare errors with no context. Callers could not distinguish between file-not-found, permission denied, and parse errors using `errors.Is` / `errors.As`.

**`cmd/retrieve.go` — Fixed four bugs in batch retrieval**
- Three `json.Marshal` calls discarded errors, silently writing an empty string to the `output_json` database column instead of failing visibly.
- A completed batch with no `OutputFileID` caused an infinite retry loop: the error was printed but the batch status was never updated, so every subsequent `retrieve` run re-attempted the same failed batch forever.
- When `GetBatch` failed (network error, rate limit), `LastCheckedAtUnix` was never updated, leaving no record of the poll attempt in the database.
- Error messages for `GetBatch` failures omitted the batch ID, making it impossible to identify which batch failed when multiple were active.

**`cmd/submit.go` — Fixed three bugs in batch submission**
- `json.Marshal(batch)` errors were discarded; the `raw_json` column received an empty string (explicitly present) instead of SQL NULL when marshalling failed.
- `InsertRequest` DB writes happened before `client.CreateBatch` was called. If batch creation failed, request rows were left orphaned with `batch_id IS NULL` and images were stuck in `submitted` status permanently, requiring manual database repair.
- `fu.GetFileInfo` errors were silently swallowed, substituting `time.Now()` for the file's real modification time with no warning to the user.

**`main.go` — Fixed unreachable parse error handling**
All four subcommand flag sets used `flag.ExitOnError`, which calls `os.Exit(2)` before `Parse` returns on error. The `if err := ...Parse(...); err != nil` blocks immediately following were unreachable dead code. Changed to `flag.ContinueOnError` so parse errors are handled by the application.

**Tests added**
Zero test coverage existed before this session. Table-driven tests with `t.Run` were added for:
- `sql/` — `CreateDB`, `OpenDB`, `CloseDB`
- `config/` — load/save round-trip, missing file, invalid TOML, `GetDBLocation`
- `openai/` — `BuildBatchRequest`, `BuildJSONL`, `ParseBatchOutput`, `ExtractContent`
- `cmd/` — `buildImageURL` (5 cases)
