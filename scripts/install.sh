#!/usr/bin/env sh
# Cross-platform installer entry point. Detects the OS and hands off to the
# matching subscript, which installs the C/OpenGL build deps, generates bundled
# assets, and builds the binary into ./bin.
#
#   ./scripts/install.sh
#
# On native Windows (PowerShell/cmd) run scripts\install-windows.ps1 directly.
set -eu

dir=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)

case "$(uname -s)" in
  Linux*)  exec sh "$dir/install-linux.sh" ;;
  Darwin*) exec sh "$dir/install-macos.sh" ;;
  MINGW* | MSYS* | CYGWIN*)
    exec powershell -ExecutionPolicy Bypass -File "$dir/install-windows.ps1" ;;
  *)
    echo "Unsupported platform: $(uname -s)" >&2
    exit 1 ;;
esac
