#!/usr/bin/env sh
# Linux: install gcc + the X11/OpenGL headers Fyne (CGO) needs, then build.
set -eu

command -v go >/dev/null 2>&1 || {
  echo "Go is not installed. Get it from https://go.dev/dl/ then re-run." >&2
  exit 1
}

# ponytail: cover the three common package managers; add others if a distro needs it.
if command -v apt-get >/dev/null 2>&1; then
  sudo apt-get update
  sudo apt-get install -y gcc libgl1-mesa-dev xorg-dev
elif command -v dnf >/dev/null 2>&1; then
  sudo dnf install -y gcc mesa-libGL-devel libXcursor-devel libXrandr-devel \
    libXinerama-devel libXi-devel libxkbcommon-devel
elif command -v pacman >/dev/null 2>&1; then
  sudo pacman -S --needed --noconfirm gcc mesa libxcursor libxrandr libxinerama libxi
else
  echo "No supported package manager (apt/dnf/pacman) found." >&2
  echo "Install gcc and the OpenGL/X11 dev headers manually, then re-run." >&2
  exit 1
fi

cd "$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
CGO_ENABLED=1 go run ./tools/genassets
CGO_ENABLED=1 go build -o bin/system-monitor ./cmd/system-monitor
echo "Built ./bin/system-monitor"
