# What's New

## System Monitor

Thanks for updating. Here's what changed recently.

### Updates: checks now run while the app is open
- The app checks for a new version at launch and then every 5 minutes, so a
  release that ships while System Monitor is running shows up in the status
  bar without a restart.
- The cadence is a **Settings → Update checks** dropdown — **Off** / 5 / 15 /
  30 / 60 minutes. **Off** disables checks entirely: the app never queries
  GitHub during the session.
- The check skips while an update is already shown, so it never re-pings the
  rate-limited API behind the banner. Auto-install (when enabled) keeps
  working off the periodic check.
