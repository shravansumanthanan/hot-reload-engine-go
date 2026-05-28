# ────────────────────────────────────────────────────────────────────────────
# hotreload — Makefile
# ────────────────────────────────────────────────────────────────────────────

VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS     := -ldflags "-X main.version=$(VERSION)"
BINARY      := hotreload

.PHONY: build test test-race lint vet fmt version demo demo-crash clean help

## build: Compile the hotreload binary.
build:
	go build $(LDFLAGS) -o $(BINARY) .

## test: Run all unit tests.
test:
	go test ./...

## test-race: Run all unit tests with the race detector.
test-race:
	go test -race ./...

## lint: Run golangci-lint (install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest).
lint:
	golangci-lint run ./...

## vet: Run go vet.
vet:
	go vet ./...

## fmt: Run gofmt.
fmt:
	gofmt -w .

## version: Print the current version string.
version:
	@echo $(VERSION)

## demo: Build and run the live-reload proxy demo against the test server.
demo: build
	./$(BINARY) --root ./testserver --proxy 8080:8081 \
		--build "go build -o ./bin/testserver ./testserver/main.go" \
		--exec "PORT=8081 ./bin/testserver"

## demo-crash: Build and run the demo in crash-mode (tests backoff logic).
demo-crash: build
	./$(BINARY) --root ./testserver --proxy 8080:8081 \
		--build "go build -o ./bin/testserver ./testserver/main.go" \
		--exec "PORT=8081 ./bin/testserver -crash-mode"

## clean: Remove build artifacts.
clean:
	rm -rf bin/ $(BINARY) *.log *.tmp

## help: Print available make targets.
help:
	@grep -E '^## ' Makefile | sed 's/## //'
