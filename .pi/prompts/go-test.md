---
description: Write table-driven Go tests with race detector
argument-hint: "[function-or-package]"
---
Write Go tests for $@ following these rules:

- Use table-driven tests with `t.Run` for subtests
- Test edge cases: zero values, nil inputs, empty slices, large inputs
- Use `t.Parallel()` where safe
- Test error paths explicitly
- Use `go test -race -count=1` to verify
- If mocking is needed, prefer interfaces over code generation
- Coverage target: meaningful paths, not 100% blindly

Do NOT use testify or external assertion libraries — stick to `if got != want { t.Errorf(...) }`.
