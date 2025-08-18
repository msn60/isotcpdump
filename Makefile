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

brun: clean build
	@$(BIN_DIR)/$(BINARY_NAME)

run: 
	@$(BIN_DIR)/$(BINARY_NAME)

clear-log:
	@mkdir -p logs
	@> logs/app.log
	@echo "✅ app.log is empty now"

rm-app-logs:
	@if ls -d logs/app-*/ 2>/dev/null; then \
		rm -rf logs/app-*/; \
		echo "✅ All app-* log folders removed."; \
	else \
		echo "ℹ️ No app-* log folders found in logs/"; \
	fi

debug:
	go build -gcflags "all=-N -l" -o $(BIN_DIR)/$(BINARY_NAME)_debug $(SRC_DIR)/$(CMD_DIR)/main.go

