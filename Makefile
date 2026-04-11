BINARY   := modbussim
WEB_DIR  := web
DIST_DIR := internal/frontend/dist
GO       := go

.PHONY: all build build-frontend build-backend run clean dev help \
        build-windows build-windows-arm64 \
        build-linux build-linux-arm64 \
        build-darwin build-darwin-arm64 \
        build-all

## all: build frontend + backend (OS/arch atual)
all: build

## build: compila frontend + binário para o sistema atual
build: build-frontend build-backend

## build-frontend: compila o React em internal/frontend/dist
build-frontend:
	@echo "→ Building frontend..."
	cd $(WEB_DIR) && npm install --silent && npm run build
	@echo "→ Copying dist to $(DIST_DIR)..."
	rm -rf $(DIST_DIR)
	cp -r $(WEB_DIR)/dist $(DIST_DIR)

## build-backend: compila o binário Go para o sistema atual
build-backend:
	@echo "→ Building Go binary ($(shell go env GOOS)/$(shell go env GOARCH))..."
	$(GO) build -o $(BINARY) ./cmd/modbussim/
	@echo "→ Done: ./$(BINARY)"

# ── Cross-compilation ────────────────────────────────────────────────────────

## build-windows: cross-compile para Windows amd64 (x86-64)
build-windows: build-frontend
	@echo "→ Cross-compiling windows/amd64..."
	GOOS=windows GOARCH=amd64 $(GO) build -o $(BINARY)-windows-amd64.exe ./cmd/modbussim/
	@echo "→ Done: $(BINARY)-windows-amd64.exe"

## build-windows-arm64: cross-compile para Windows ARM64 (Surface, Snapdragon X)
build-windows-arm64: build-frontend
	@echo "→ Cross-compiling windows/arm64..."
	GOOS=windows GOARCH=arm64 $(GO) build -o $(BINARY)-windows-arm64.exe ./cmd/modbussim/
	@echo "→ Done: $(BINARY)-windows-arm64.exe"

## build-linux: cross-compile para Linux amd64
build-linux: build-frontend
	@echo "→ Cross-compiling linux/amd64..."
	GOOS=linux GOARCH=amd64 $(GO) build -o $(BINARY)-linux-amd64 ./cmd/modbussim/
	@echo "→ Done: $(BINARY)-linux-amd64"

## build-linux-arm64: cross-compile para Linux ARM64 (Raspberry Pi 4/5, servidores ARM)
build-linux-arm64: build-frontend
	@echo "→ Cross-compiling linux/arm64..."
	GOOS=linux GOARCH=arm64 $(GO) build -o $(BINARY)-linux-arm64 ./cmd/modbussim/
	@echo "→ Done: $(BINARY)-linux-arm64"

## build-darwin: cross-compile para macOS Intel (amd64)
build-darwin: build-frontend
	@echo "→ Cross-compiling darwin/amd64..."
	GOOS=darwin GOARCH=amd64 $(GO) build -o $(BINARY)-darwin-amd64 ./cmd/modbussim/
	@echo "→ Done: $(BINARY)-darwin-amd64"

## build-darwin-arm64: cross-compile para macOS Apple Silicon (M1/M2/M3/M4)
build-darwin-arm64: build-frontend
	@echo "→ Cross-compiling darwin/arm64..."
	GOOS=darwin GOARCH=arm64 $(GO) build -o $(BINARY)-darwin-arm64 ./cmd/modbussim/
	@echo "→ Done: $(BINARY)-darwin-arm64"

## build-all: compila frontend uma vez + binários para todas as plataformas
build-all: build-frontend
	@echo "→ Building all platforms..."
	GOOS=windows GOARCH=amd64 $(GO) build -o $(BINARY)-windows-amd64.exe ./cmd/modbussim/
	GOOS=windows GOARCH=arm64 $(GO) build -o $(BINARY)-windows-arm64.exe  ./cmd/modbussim/
	GOOS=linux   GOARCH=amd64 $(GO) build -o $(BINARY)-linux-amd64        ./cmd/modbussim/
	GOOS=linux   GOARCH=arm64 $(GO) build -o $(BINARY)-linux-arm64         ./cmd/modbussim/
	GOOS=darwin  GOARCH=amd64 $(GO) build -o $(BINARY)-darwin-amd64        ./cmd/modbussim/
	GOOS=darwin  GOARCH=arm64 $(GO) build -o $(BINARY)-darwin-arm64         ./cmd/modbussim/
	@echo ""
	@echo "→ All builds complete:"
	@ls -lh $(BINARY)-* 2>/dev/null || dir $(BINARY)-*

# ── Desenvolvimento ──────────────────────────────────────────────────────────

## run: build completo + inicia o simulador
run: build
	./$(BINARY)

## dev-frontend: inicia o Vite dev server (proxy para :7070)
dev-frontend:
	cd $(WEB_DIR) && npm run dev

## clean: remove binários e dist (restaura stub do frontend)
clean:
	rm -f $(BINARY) $(BINARY)-windows-amd64.exe $(BINARY)-windows-arm64.exe \
	      $(BINARY)-linux-amd64 $(BINARY)-linux-arm64 \
	      $(BINARY)-darwin-amd64 $(BINARY)-darwin-arm64
	rm -rf $(DIST_DIR)
	@mkdir -p $(DIST_DIR)
	@printf '<!DOCTYPE html><html><head><title>ModbusSim</title></head><body>Run <code>make build</code> to compile the frontend.</body></html>\n' > $(DIST_DIR)/index.html

## help: exibe esta ajuda
help:
	@grep -E '^##' Makefile | sed 's/## //'
