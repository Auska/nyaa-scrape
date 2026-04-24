# Nyaa Crawler Makefile

# Go environment
export PATH := /usr/local/go/bin:$(PATH)
GOPATH := $(shell go env GOPATH)
export PATH := $(GOPATH)/bin:$(PATH)

# Build variables
VERSION := $(shell date +%Y%m%d)
OUTPUT_DIR := release
LDFLAGS := -ldflags="-s -w"

# Go files
GO_FILES := $(shell find . -name '*.go' -type f)

.PHONY: all build build-linux build-windows build-macos test lint clean run query help

all: lint test build

## build: Build all platforms
build: build-linux build-windows build-macos

## build-linux: Build for Linux amd64
build-linux:
	@echo "Building for Linux amd64..."
	@mkdir -p $(OUTPUT_DIR)
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(OUTPUT_DIR)/nyaa-crawler-linux-amd64 ./cmd/crawler
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(OUTPUT_DIR)/nyaa-query-linux-amd64 ./cmd/query
	cd $(OUTPUT_DIR) && tar -czf nyaa-crawler-linux-amd64-$(VERSION).tar.gz nyaa-crawler-linux-amd64 nyaa-query-linux-amd64
	@echo "Linux build complete: $(OUTPUT_DIR)/nyaa-crawler-linux-amd64-$(VERSION).tar.gz"

## build-windows: Build for Windows amd64
build-windows:
	@echo "Building for Windows amd64..."
	@mkdir -p $(OUTPUT_DIR)
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(OUTPUT_DIR)/nyaa-crawler-windows-amd64.exe ./cmd/crawler
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(OUTPUT_DIR)/nyaa-query-windows-amd64.exe ./cmd/query
	cd $(OUTPUT_DIR) && tar -czf nyaa-crawler-windows-amd64-$(VERSION).tar.gz nyaa-crawler-windows-amd64.exe nyaa-query-windows-amd64.exe
	@echo "Windows build complete: $(OUTPUT_DIR)/nyaa-crawler-windows-amd64-$(VERSION).tar.gz"

## build-macos: Build for macOS amd64
build-macos:
	@echo "Building for macOS amd64..."
	@mkdir -p $(OUTPUT_DIR)
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(OUTPUT_DIR)/nyaa-crawler-macos-amd64 ./cmd/crawler
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o $(OUTPUT_DIR)/nyaa-query-macos-amd64 ./cmd/query
	cd $(OUTPUT_DIR) && tar -czf nyaa-crawler-macos-amd64-$(VERSION).tar.gz nyaa-crawler-macos-amd64 nyaa-query-macos-amd64
	@echo "macOS build complete: $(OUTPUT_DIR)/nyaa-crawler-macos-amd64-$(VERSION).tar.gz"

## test: Run all tests
test:
	@echo "Running tests..."
	go test -v -race -cover ./...

## lint: Run golangci-lint
lint:
	@echo "Running linter..."
	golangci-lint run ./...

## fmt: Format code
fmt:
	@echo "Formatting code..."
	go fmt ./...

## clean: Remove build artifacts
clean:
	@echo "Cleaning..."
	rm -rf $(OUTPUT_DIR)
	go clean ./...

## run: Run the crawler
run:
	go run ./cmd/crawler

## query: Run the query tool
query:
	go run ./cmd/query

## help: Show this help
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@sed -n 's/^## //p' $(MAKEFILE_LIST) | column -t -s ':'
