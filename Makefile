BINARY   := modbussim
WEB_DIR  := web
DIST_DIR := internal/frontend/dist
GO       := go

.PHONY: all build build-frontend build-backend run clean dev

## all: build frontend + backend
all: build

## build: build frontend then backend (produces ./modbussim)
build: build-frontend build-backend

## build-frontend: compile React app into internal/frontend/dist
build-frontend:
	@echo "→ Building frontend..."
	cd $(WEB_DIR) && npm install --silent && npm run build
	@echo "→ Copying dist to $(DIST_DIR)..."
	rm -rf $(DIST_DIR)
	cp -r $(WEB_DIR)/dist $(DIST_DIR)

## build-backend: compile Go binary
build-backend:
	@echo "→ Building Go binary..."
	$(GO) build -o $(BINARY) ./cmd/modbussim/

## run: build everything and start the simulator
run: build
	./$(BINARY)

## dev-frontend: start Vite dev server (proxies API to :7070)
dev-frontend:
	cd $(WEB_DIR) && npm run dev

## clean: remove binary and dist
clean:
	rm -f $(BINARY)
	rm -rf $(DIST_DIR)

## help: show this help
help:
	@grep -E '^##' Makefile | sed 's/## //'
