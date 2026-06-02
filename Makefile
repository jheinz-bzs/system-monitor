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

.PHONY: run start build build-win vet tidy fmt clean

## run: build and launch the application
run start:
	go run $(PKG)

## build: compile the binary into ./bin
build:
	go build -o $(BIN) $(PKG)

## build-win: compile a windowed Windows binary (no console window)
build-win:
	go build -ldflags="-H windowsgui" -o $(BIN).exe $(PKG)

## vet: run go vet across all packages
vet:
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
