.PHONY: build test clean install e2e e2e-live

BINARY=apex
BUILD_DIR=bin

build:
	go build -o $(BUILD_DIR)/$(BINARY) ./cmd/apex/

test:
	go test ./... -v -count=1

test-coverage:
	go test ./... -v -count=1 -coverprofile=coverage.out
	go tool cover -func=coverage.out

clean:
	rm -rf $(BUILD_DIR) coverage.out

install: build
	cp $(BUILD_DIR)/$(BINARY) /usr/local/bin/$(BINARY)

e2e:
	go test ./e2e/... -v -count=1 -timeout=120s

e2e-live:
	APEX_LIVE_TESTS=1 go test ./e2e/... -v -count=1 -tags=live -timeout=300s
