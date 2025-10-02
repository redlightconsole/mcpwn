.PHONY: all build clean run-api run-mcp run help

GOCMD := go
GOBUILD := $(GOCMD) build

BIN_DIR := ./bin
CMD_DIR := ./cmd

SERVICES := api-server mcp-server

bin_path = $(BIN_DIR)/$(1)
src_path = $(CMD_DIR)/$(1)

all: build

build: $(SERVICES)

$(SERVICES): %:
	@echo "Building $@..."
	@mkdir -p $(BIN_DIR)
	$(GOBUILD) -o $(call bin_path,$@) $(call src_path,$@)

run-api: api-server
	@echo "Starting api-server on port 5000..."
	$(call bin_path,api-server) --port 5000

run-mcp: mcp-server
	@echo "Starting mcp-server on port 8000..."
	$(call bin_path,mcp-server) --port 8000 --server "http://localhost:5000"

clean:
	@echo "Cleaning binaries..."
	@rm -rf $(BIN_DIR)