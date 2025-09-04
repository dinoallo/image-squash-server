# Simple Makefile for image-manip CLI
# Generated to build binaries into bin/

MODULE := github.com/lingdie/image-manip-server
BINARY := image-manip
CMD_DIR := ./cmd/cli
OUT_DIR := bin
GO ?= go

# Default target
.PHONY: all
all: build

.PHONY: build
build: $(OUT_DIR)/$(BINARY)

$(OUT_DIR)/$(BINARY): $(shell find . -name '*.go' -not -path './vendor/*')
	@mkdir -p $(OUT_DIR)
	$(GO) build -o $(OUT_DIR)/$(BINARY) $(CMD_DIR)
	@echo "Built $(OUT_DIR)/$(BINARY)"

.PHONY: build-all
# Build for multiple platforms (add more as needed)
build-all:
	@mkdir -p $(OUT_DIR)
	GOOS=linux GOARCH=amd64 $(GO) build -o $(OUT_DIR)/$(BINARY)-linux-amd64 $(CMD_DIR)
	GOOS=linux GOARCH=arm64 $(GO) build -o $(OUT_DIR)/$(BINARY)-linux-arm64 $(CMD_DIR)
	GOOS=darwin GOARCH=arm64 $(GO) build -o $(OUT_DIR)/$(BINARY)-darwin-arm64 $(CMD_DIR)
	GOOS=darwin GOARCH=amd64 $(GO) build -o $(OUT_DIR)/$(BINARY)-darwin-amd64 $(CMD_DIR)
	@echo "Built multi-platform binaries in $(OUT_DIR)/"

.PHONY: test
test:
	$(GO) test ./...

.PHONY: tidy
tidy:
	$(GO) mod tidy

.PHONY: clean
clean:
	rm -rf $(OUT_DIR)

.PHONY: run
run: build
	./$(OUT_DIR)/$(BINARY) --help

.PHONY: fmt
fmt:
	$(GO) fmt ./...

.PHONY: vet
vet:
	$(GO) vet ./...

.PHONY: lint
lint: vet fmt

# Convenience target to ensure code compiles without producing artifact
.PHONY: check
check:
	$(GO) build ./...
