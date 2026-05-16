.PHONY: all build build-go build-rust build-python lint test fmt clean snapshot

all: build

# Build
build: build-go build-rust build-python

build-go:
	cd go && for dir in cmd/*/; do [ -d "$$dir" ] && go build -o ../dist/ ./$$dir || true; done

build-rust:
	cd rust && cargo build --release --workspace

build-python:
	cd python && pip install -e .

# Lint
lint:
	cd go && golangci-lint run ./...
	cd rust && cargo clippy --workspace -- -D warnings
	cd python && ruff check .
	shellcheck scripts/bin/*.sh 2>/dev/null || true

# Test
test:
	cd go && CGO_ENABLED=1 go test ./...
	cd rust && cargo test --workspace
	cd python && pytest

# Format
fmt:
	cd go && gofmt -w .
	cd rust && cargo fmt
	cd python && ruff format .

## snapshot: build a local snapshot for all four targets using zig cc for Linux cross-compilation
## Requires: goreleaser (go install github.com/goreleaser/goreleaser/v2@latest)
##           zig (brew install zig) for Linux cross-compilation on macOS
snapshot:
	CC_FOR_LINUX_AMD64="zig cc -target x86_64-linux-musl" \
	CC_FOR_LINUX_ARM64="zig cc -target aarch64-linux-musl" \
	goreleaser release --snapshot --clean

# Clean
clean:
	rm -rf dist/*
	cd rust && cargo clean 2>/dev/null || true
	find python -type d -name __pycache__ -exec rm -rf {} + 2>/dev/null || true
