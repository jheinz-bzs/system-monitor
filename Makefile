# System Monitor — common dev tasks.
#
# Fyne requires CGO and a C compiler (mingw-w64 gcc on Windows), so CGO_ENABLED
# is forced on here.
#
# Usage (GNU make; on Windows the WinLibs toolchain ships `mingw32-make`):
#   make run      # build and launch the app  (npm-start equivalent)
#   make build    # compile the binary into ./bin
#   make vet      # static analysis
#   make tidy     # sync go.mod / go.sum
#   make fmt      # gofmt the tree

export CGO_ENABLED := 1

PKG := ./cmd/system-monitor
BIN := bin/system-monitor

# Bundled assets are compiled into Go source (no //go:embed). ASSETS_GEN is
# regenerated whenever a font/icon file or the generator changes.
ASSETS_GEN := internal/ui/assets_gen.go
ASSET_SRC  := $(wildcard internal/ui/fonts/*.ttf internal/ui/icons/*.svg) tools/genassets/main.go

.PHONY: run start build build-win build-record release vet tidy fmt clean generate

$(ASSETS_GEN): $(ASSET_SRC)
	go run ./tools/genassets

## generate: compile bundled assets into internal/ui/assets_gen.go
generate: $(ASSETS_GEN)

## run: build and launch the application
run start: $(ASSETS_GEN)
	go run $(PKG)

## build: compile the binary into ./bin
build: $(ASSETS_GEN)
	go build -o $(BIN) $(PKG)

## build-win: compile a windowed Windows binary (no console window)
build-win: $(ASSETS_GEN)
	go build -ldflags="-H windowsgui" -o $(BIN).exe $(PKG)

## build-record: compile the headless recording agent (no Fyne, no cgo, no
## generated assets). Cross-compiling is just prefixing GOOS/GOARCH, e.g.
## GOOS=linux GOARCH=amd64 make build-record
build-record: export CGO_ENABLED := 0
build-record:
	go build -o $(BIN)-record ./cmd/system-monitor-record

## release: build a versioned Windows release binary locally (usage: VERSION=v1.2.3 make release)
## Mirrors the ldflags the release workflow uses, for a hand-cut release or a swap test.
release: $(ASSETS_GEN)
	go build -ldflags="-H windowsgui -X main.version=$(VERSION)" -o $(BIN)-$(VERSION).exe $(PKG)

## vet: run go vet across all packages
vet: $(ASSETS_GEN)
	go vet ./...

## tidy: add missing and remove unused modules
tidy:
	go mod tidy

## fmt: format all Go source
fmt:
	gofmt -w .

## clean: remove build artifacts
clean:
	go clean
	rm -rf bin
