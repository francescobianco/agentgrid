BINARY=agentgrid
BUILD_DIR=bin

.PHONY: start build clean dev

start: data/agentgrid.db
	@if [ ! -f .env ]; then cp .env.example .env; echo "==> Created .env from .env.example"; fi
	@echo "==> Starting AgentGrid..."
	go run .

build:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY) .

dev: start

clean:
	rm -rf $(BUILD_DIR) data/

data/agentgrid.db:
	@mkdir -p data
	@touch data/agentgrid.db

push:
	@git add .
	@git commit -m "Update" || true
	@git push