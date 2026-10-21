.PHONY: all build build-darwin build-darwin-universal build-windows test lint doctor demo clean run verify setup-hooks

BINARY_NAME=pptx-remote
BUILD_DIR=bin
VERSION?=$(shell git describe --tags --always 2>/dev/null || echo "0.0.1")

all: build

build: build-darwin

build-darwin:
	@mkdir -p $(BUILD_DIR)
	go build -ldflags "-s -w -X main.version=$(VERSION)" -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/pptx-remote
	@echo "Built $(BUILD_DIR)/$(BINARY_NAME) (macOS)"

build-darwin-universal:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "-s -w -X main.version=$(VERSION)" -o $(BUILD_DIR)/$(BINARY_NAME)-arm64 ./cmd/pptx-remote
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w -X main.version=$(VERSION)" -o $(BUILD_DIR)/$(BINARY_NAME)-amd64 ./cmd/pptx-remote
	lipo -create -output $(BUILD_DIR)/$(BINARY_NAME) $(BUILD_DIR)/$(BINARY_NAME)-arm64 $(BUILD_DIR)/$(BINARY_NAME)-amd64
	@rm $(BUILD_DIR)/$(BINARY_NAME)-arm64 $(BUILD_DIR)/$(BINARY_NAME)-amd64
	@echo "Built Universal Darwin Binary at $(BUILD_DIR)/$(BINARY_NAME)"

build-windows:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-s -w -H=windowsgui -X main.version=$(VERSION)" -o $(BUILD_DIR)/$(BINARY_NAME).exe ./cmd/pptx-remote
	@echo "Built $(BUILD_DIR)/$(BINARY_NAME).exe (Windows, GUI mode)"

build-all: build-darwin-universal build-windows

setup-hooks:
	git config core.hooksPath .githooks
	chmod +x .githooks/*
	@echo "Git hooks configured to .githooks/"

verify:
	@./scripts/agent-verify.sh

test:
	go test -v ./...

lint:
	@test -z "$$(gofmt -l .)" || (echo "Unformatted Go files: $$(gofmt -l .)" && exit 1)
	go vet ./...
	node -c internal/webui/static/app.js internal/webui/static/crypto.js internal/webui/static/sw.js

doctor: build-darwin
	./$(BUILD_DIR)/$(BINARY_NAME) doctor

demo: build-darwin
	./$(BUILD_DIR)/$(BINARY_NAME) -mode local -no-browser -demo internal/pptx/testdata/demo.pptx

run: build-darwin
	./$(BUILD_DIR)/$(BINARY_NAME)

package-macos:
	@./scripts/package-macos.sh

package-windows:
	@./scripts/package-windows.sh

package-linux:
	@./scripts/package-linux.sh

package: package-macos package-windows package-linux

clean:
	rm -rf $(BUILD_DIR) dist build
