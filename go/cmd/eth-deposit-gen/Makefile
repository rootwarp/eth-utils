.PHONY: test lint build

test:
	go test ./...

lint:
	go vet ./...
	staticcheck ./...

build:
	go build -o bin/eth-deposit-gen .
