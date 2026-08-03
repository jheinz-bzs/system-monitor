# What's New

## System Monitor

Thanks for updating. Here's what changed recently.

### Linux: updates via apt now get a proper prompt
- If you installed from the `.deb`, an available update now shows the update
  banner and an **Update via apt** link — tapping it tells you to run
  `sudo apt update && sudo apt upgrade` instead of the app swapping a file it
  doesn't own. Standalone binaries and AppImages still self-update in place.

### macOS: installs are smoother
- The Homebrew cask now clears the quarantine attribute automatically after
  install, so the app installed by a fresh `brew install --cask system-monitor`
  launches without a Gatekeeper prompt.
