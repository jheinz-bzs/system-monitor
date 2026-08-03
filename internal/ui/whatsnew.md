# What's New

## System Monitor

Thanks for updating. Here's what changed recently.

### macOS installs actually work now
- The app bundle is now signed before it ships, so Apple Silicon no longer
  refuses to launch it with the "damaged" error. First launch may still ask
  you to approve it (the app is ad-hoc signed, not notarized) — see
  `docs/install/INSTALL-MACOS.md` for the one-time Gatekeeper bypass.
- Self-updates re-sign the bundle after swapping in the new version, so an
  in-app update won't re-break the install.
