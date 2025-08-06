BINARY_NAME=isotcp
SRC_DIR=.
BIN_DIR=bin
CMD_DIR=cmd

.PHONY: build run clean

clean:
	rm -rf $(BIN_DIR)
	@echo "🧹 Cleaned up build artifacts"

build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(BINARY_NAME) $(SRC_DIR)/$(CMD_DIR)/main.go
	@echo "✅ Built binary at $(BIN_DIR)/$(BINARY_NAME)"

run: clean build
	@$(BIN_DIR)/$(BINARY_NAME)

debug:
	go build -gcflags "all=-N -l" -o $(BIN_DIR)/$(BINARY_NAME)_debug $(SRC_DIR)/$(CMD_DIR)/main.go


