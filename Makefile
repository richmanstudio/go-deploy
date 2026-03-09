APP_NAME := go-deploy
BIN_DIR := bin
CMD_PATH := ./cmd/go-deploy

ifeq ($(OS),Windows_NT)
BIN_EXT := .exe
else
BIN_EXT :=
endif

.PHONY: build run test lint clean

build:
	@mkdir -p $(BIN_DIR)
	go build -o $(BIN_DIR)/$(APP_NAME)$(BIN_EXT) $(CMD_PATH)

run:
	go run $(CMD_PATH)

test:
	go test ./...

lint:
	go vet ./...

clean:
	rm -rf $(BIN_DIR)
