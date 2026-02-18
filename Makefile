.PHONY: build test clean install

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
