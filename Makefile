# otb — Makefile
#
# Security build flags:
#   CGO_ENABLED=0   — pure Go, no C runtime, fully static binary
#   -trimpath       — removes local build paths from binary (no source leakage)
#   -ldflags="-s -w" — strip symbol table and DWARF debug info (smaller + no paths)

GO       := go
BINARY   := bin/otb
MODULE   := github.com/pot-labs/otb
LDFLAGS  := -s -w
GOFLAGS  := -trimpath -ldflags="$(LDFLAGS)"

.PHONY: all build test vet lint security clean install

all: vet test build

## build: compile static binary for linux/amd64
build:
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build $(GOFLAGS) -o $(BINARY) .
	@echo "✓ built $(BINARY) ($$(du -sh $(BINARY) | cut -f1))"

## build-arm64: cross-compile for linux/arm64
build-arm64:
	@mkdir -p bin
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) build $(GOFLAGS) -o bin/otb-arm64 .
	@echo "✓ built bin/otb-arm64"

## test: run all tests with race detector
test:
	$(GO) test -race -count=1 -timeout=120s ./...

## vet: run go vet
vet:
	$(GO) vet ./...

## lint: run staticcheck (install with: go install honnef.co/go/tools/cmd/staticcheck@latest)
lint:
	@which staticcheck > /dev/null 2>&1 && staticcheck ./... || echo "staticcheck not found — skipping (install: go install honnef.co/go/tools/cmd/staticcheck@latest)"

## security: run govulncheck (install with: go install golang.org/x/vuln/cmd/govulncheck@latest)
security:
	@which govulncheck > /dev/null 2>&1 && govulncheck ./... || echo "govulncheck not found — skipping (install: go install golang.org/x/vuln/cmd/govulncheck@latest)"

## checksums: generate sha256 checksums for release binaries
checksums:
	@cd bin && sha256sum otb* > checksums.sha256 && cat checksums.sha256

## clean: remove build artifacts
clean:
	rm -rf bin/

## install: install binary to /usr/local/bin (requires write permission)
install: build
	cp $(BINARY) /usr/local/bin/otb
	@echo "✓ installed otb to /usr/local/bin/otb"

## install-hooks: install git hooks from scripts/hooks/ into .git/hooks/
install-hooks:
	@cp scripts/hooks/pre-push .git/hooks/pre-push
	@chmod +x .git/hooks/pre-push
	@echo "✓ pre-push hook installed"

## mod-tidy: tidy go modules
mod-tidy:
	$(GO) mod tidy


## test-docker-build: build the test Docker image
test-docker-build:
	docker build -f Dockerfile.test -t otb-tests .

## test-docker: run unit + security tests in Docker
test-docker: test-docker-build
	docker run --rm \
	  -e TEST_MODE=all \
	  -v $$(pwd)/test-results:/results \
	  --read-only \
	  --tmpfs /tmp:size=256m \
	  --tmpfs /root/.cache:size=512m \
	  otb-tests

## test-docker-fuzz: run fuzz tests in Docker (default 30s per target)
test-docker-fuzz: test-docker-build
	docker run --rm \
	  -e TEST_MODE=fuzz \
	  -e FUZZ_SECONDS=$${FUZZ_SECONDS:-30} \
	  -v $$(pwd)/test-results:/results \
	  --read-only \
	  --tmpfs /tmp:size=256m \
	  --tmpfs /root/.cache:size=1g \
	  otb-tests

## test-docker-full: unit + fuzz + security in Docker
test-docker-full: test-docker-build
	docker run --rm \
	  -e TEST_MODE=fuzz+security \
	  -e FUZZ_SECONDS=$${FUZZ_SECONDS:-30} \
	  -v $$(pwd)/test-results:/results \
	  --read-only \
	  --tmpfs /tmp:size=256m \
	  --tmpfs /root/.cache:size=1g \
	  otb-tests

## help: show this help
help:
	@grep -E '^## ' Makefile | sed 's/## /  /'
