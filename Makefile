.PHONY: build run clean deps install fmt vet test all examples

BINARY_NAME := brutus
GO := go
GOPATH := $(shell $(GO) env GOPATH)

ifeq ($(OS),Windows_NT)
	BINARY_NAME := brutus.exe
	RM := del /Q
	INSTALL_DIR := $(GOPATH)/bin
else
	RM := rm -f
	INSTALL_DIR := $(GOPATH)/bin
endif

build:
	@echo "Building BRUTUS..."
	$(GO) build -o $(BINARY_NAME) .

run: build
	./$(BINARY_NAME)

run-verbose: build
	./$(BINARY_NAME) -verbose

clean:
	@echo "Cleaning..."
	$(RM) $(BINARY_NAME)

deps:
	@echo "Installing dependencies..."
	$(GO) mod tidy
	$(GO) mod download

install: build
	@echo "Installing BRUTUS to $(INSTALL_DIR)..."
	@mkdir -p $(INSTALL_DIR) 2>/dev/null || true
	cp $(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "Installed! Make sure $(INSTALL_DIR) is in your PATH."
	@echo "Then run 'brutus' from any directory."

fmt:
	@echo "Formatting code..."
	$(GO) fmt ./...

vet:
	@echo "Running go vet..."
	$(GO) vet ./...

test:
	@echo "Running tests..."
	$(GO) test -v ./...

examples:
	@echo "Building examples..."
	$(GO) build -o examples/01-chat/chat ./examples/01-chat
	$(GO) build -o examples/02-read-tool/read-tool ./examples/02-read-tool
	$(GO) build -o examples/03-multi-tool/multi-tool ./examples/03-multi-tool

all: deps fmt vet build

.DEFAULT_GOAL := build
