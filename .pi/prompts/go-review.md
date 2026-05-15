---
description: Review Go code for bugs, concurrency issues, and performance
argument-hint: "[file-or-package]"
---
Review the Go code $@ for:

- Bugs and logic errors
- Concurrency issues (race conditions, deadlocks, goroutine leaks)
- Error handling gaps (unchecked errors, swallowed context cancellations)
- Performance concerns (unnecessary allocations, inefficient data structures)
- Go idiom violations (use sync.Pool where appropriate, prefer io.Reader over []byte copies)
- Missing or inadequate tests

Suggest specific fixes. Always use `%w` for error wrapping.
