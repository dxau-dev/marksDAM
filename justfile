# Build for current platform
build:
    go build -o marksdam .

# Build for all supported platforms (requires pure-Go sqlite driver — no CGo needed)
build-all:
    go build -o marksdam-darwin-arm64 .
    GOOS=linux GOARCH=amd64 go build -o marksdam-linux-amd64 .

# Run all tests
test:
    go test ./...

# Run tests with race detector
test-race:
    go test -race ./...

# Lint
lint:
    golangci-lint run

# Format source files
fmt:
    gofmt -w . && goimports -w .

# Regenerate sqlc query code
sqlc:
    cd sql && sqlc generate
