# marksDAM — Go Project

Module: `github.com/dxau-dev/marksDAM` · Go 1.25.6

## Commands
- **Build:** `go build ./...`
- **Run:** `go run .`
- **Test:** `go test ./...`
- **Test with race:** `go test -race ./...`
- **Lint:** `golangci-lint run`
- **Format:** `gofmt -w . && goimports -w .`
- **Vet:** `go vet ./...`

## Style & Idioms
- Error wrapping: always `fmt.Errorf("doing X: %w", err)`
- `context.Context` as first parameter
- Use `errors.Is` / `errors.As`, not `==` comparison
- Table-driven tests with `t.Run`, no testify
- Doc comments on every exported name, starting with the name

## Project Structure
[describe your packages and their responsibilities here]
