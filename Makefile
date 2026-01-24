.PHONY: all build build-go build-rust build-python lint test fmt clean

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
	cd go && go test ./...
	cd rust && cargo test --workspace
	cd python && pytest

# Format
fmt:
	cd go && gofmt -w .
	cd rust && cargo fmt
	cd python && ruff format .

# Clean
clean:
	rm -rf dist/*
	cd rust && cargo clean 2>/dev/null || true
	find python -type d -name __pycache__ -exec rm -rf {} + 2>/dev/null || true
