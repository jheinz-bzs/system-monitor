# Installing on Linux

All release assets are **x86_64 (amd64)** and are listed on the
[latest release](https://github.com/josephheinz/system-monitor/releases/latest)
page. Verify any download against `checksums.txt`:

```sh
sha256sum -c --ignore-missing checksums.txt
```

Pick one of the packaged formats, or build from source.

## Option 1 — APT repository (Debian, Ubuntu, Mint, Pop!_OS)

Best choice for Debian-family distros: add the signed repo once, then
`apt upgrade` keeps System Monitor current with the rest of your system.

```sh
# Trust the repo's signing key.
curl -fsSL https://josephheinz.github.io/system-monitor/public.key \
  | sudo gpg --dearmor --yes -o /usr/share/keyrings/system-monitor-archive-keyring.gpg
# Point apt at the repo.
echo "deb [signed-by=/usr/share/keyrings/system-monitor-archive-keyring.gpg] https://josephheinz.github.io/system-monitor/ stable main" \
  | sudo tee /etc/apt/sources.list.d/system-monitor.list
sudo apt update
sudo apt install system-monitor
```

Launch from your app menu or with `system-monitor`; updates arrive through
`sudo apt upgrade`. Uninstall with `sudo apt remove system-monitor`.

## Option 2 — AppImage (any modern distro)

Best choice for most desktops (Ubuntu, Fedora, Arch, openSUSE, …) — a single
file that bundles its GL/X11 dependencies and supports the app's built-in
self-update.

```sh
wget https://github.com/josephheinz/system-monitor/releases/latest/download/system-monitor-linux-amd64.AppImage
chmod +x system-monitor-linux-amd64.AppImage
./system-monitor-linux-amd64.AppImage
```

If it fails with a FUSE error (common on Ubuntu 22.04+), install `libfuse2`
or run with `APPIMAGE_EXTRACT_AND_RUN=1`.

## Option 3 — .deb (direct download, no setup)

For Debian-family distros that would rather not add a repo: a one-off install,
no repo setup, but no `apt upgrade` tracking either — use the APT repository
above if you want updates.

```sh
wget https://github.com/josephheinz/system-monitor/releases/latest/download/system-monitor_<version>_amd64.deb
sudo apt install ./system-monitor_<version>_amd64.deb
```

Replace `<version>` with the release version (e.g. `system-monitor_0.3.0_amd64.deb`
for `v0.3.0` — note the underscore naming and no leading `v`). Launch from your
app menu or with `system-monitor`. Uninstall with `sudo apt remove system-monitor`.

## Option 4 — Bare binary (no file extension)

For servers, minimal setups, or distros without AppImage/deb support. Unlike
the AppImage, this links GL/X11 at runtime, so your system needs Mesa/OpenGL
and X11 (or XWayland) libraries present.

```sh
wget https://github.com/josephheinz/system-monitor/releases/latest/download/system-monitor-linux-amd64
chmod +x system-monitor-linux-amd64
./system-monitor-linux-amd64
```

Optionally move it onto your PATH:

```sh
sudo install system-monitor-linux-amd64 /usr/local/bin/system-monitor
```

## Option 5 — Build from source

Requirements: **Go 1.26+**, gcc, and the GL/X11 dev headers (Fyne needs CGO).

```sh
# Debian/Ubuntu
sudo apt install gcc libgl1-mesa-dev xorg-dev
# Fedora
sudo dnf install gcc mesa-libGL-devel libXcursor-devel libXrandr-devel libXinerama-devel libXi-devel libXxf86vm-devel

git clone https://github.com/josephheinz/system-monitor.git
cd system-monitor
make build     # generates bundled assets, then builds ./bin/system-monitor
```

The `make` target runs the required asset-generation step automatically; a
bare `go build` on a fresh clone fails until you run `go run ./tools/genassets`.
