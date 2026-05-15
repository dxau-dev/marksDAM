---
description: Add or improve Go doc comments per official conventions
argument-hint: "[package-or-exported-symbol]"
---
Add or improve godoc comments for $@:

- Every exported name must have a doc comment
- Comment starts with the name of the symbol: `// Foo does X.`
- Package doc goes in a `doc.go` file or above the `package` declaration
- Document concurrency safety: "Foo is safe for concurrent use."
- Document nil behaviour: "If r is nil, returns an empty result."
- Examples in `example_test.go` where useful

Follow the [Go Doc Comments](https://go.dev/doc/comment) guide strictly.
