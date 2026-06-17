#!/usr/bin/env sh
# macOS: ensure the Xcode command-line tools (clang for CGO) are present, then build.
set -eu

command -v go >/dev/null 2>&1 || {
  echo "Go is not installed. Get it from https://go.dev/dl/ (or 'brew install go') then re-run." >&2
  exit 1
}

if ! xcode-select -p >/dev/null 2>&1; then
  echo "Installing Xcode command-line tools (a GUI prompt may appear)..."
  xcode-select --install || true
  echo "Finish the Xcode CLT install if prompted, then re-run this script." >&2
  exit 1
fi

cd "$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
CGO_ENABLED=1 go run ./tools/genassets
CGO_ENABLED=1 go build -o bin/system-monitor ./cmd/system-monitor
echo "Built ./bin/system-monitor"
