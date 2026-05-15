---
description: Refactor Go code for clarity, idiomatic patterns, and maintainability
argument-hint: "[function-or-package]"
---
Refactor this Go code $@:

- Break large functions into smaller, single-purpose ones
- Extract interfaces where it improves testability
- Replace magic numbers with named constants
- Use `errors.Is` / `errors.As` instead of direct comparison
- Prefer `fmt.Errorf` with `%w` for wrapping
- Apply Go standard library patterns (e.g. `io.Reader` chains, `bufio.Scanner`)
- Keep exported API surface clean

Preserve existing behaviour. Add tests for the new structure.
