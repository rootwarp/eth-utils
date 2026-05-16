.PHONY: test lint build refresh-golden

test:
	CGO_ENABLED=1 go test ./...

lint:
	go vet ./...
	staticcheck ./...

build:
	go build -o bin/eth-deposit-gen .

# refresh-golden regenerates testdata/hoodi/* from the fixed goldenSecret in
# test/e2e/hoodi_test.go. Run this after any change to the signing pipeline,
# SSZ implementation, or output format. Commit the updated testdata/.
refresh-golden:
	REFRESH_GOLDEN=1 CGO_ENABLED=1 go test -run TestRefreshHoodiGolden -v -timeout 120s ./test/e2e/
