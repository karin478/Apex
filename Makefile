.PHONY: build test test-race test-coverage lint vet clean install e2e e2e-live

BINARY=apex
BUILD_DIR=bin
VERSION=0.1.0

build:
	go build -o $(BUILD_DIR)/$(BINARY) -ldflags="-s -w" ./cmd/apex/

test:
	go test ./... -count=1

test-race:
	go test ./... -race -count=1

test-coverage:
	go test ./... -count=1 -coverprofile=coverage.out
	go tool cover -func=coverage.out
	@echo ""
	@echo "Coverage report: go tool cover -html=coverage.out"

lint:
	@command -v golangci-lint >/dev/null 2>&1 || { echo "Install: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; exit 1; }
	golangci-lint run ./...

vet:
	go vet ./...

clean:
	rm -rf $(BUILD_DIR) coverage.out $(BINARY)

install: build
	cp $(BUILD_DIR)/$(BINARY) /usr/local/bin/$(BINARY)

e2e:
	go test ./e2e/... -v -count=1 -timeout=120s

e2e-live:
	@# Load .env if present (contains CLAUDE_CODE_OAUTH_TOKEN)
	@test -f .env && set -a && . ./.env && set +a; \
		APEX_LIVE_TESTS=1 go test ./e2e/... -v -count=1 -tags=live -timeout=300s

# Quick check: build + vet + test
check: vet test build
	@echo "All checks passed."
