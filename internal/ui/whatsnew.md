# What's New

## System Monitor

Thanks for updating. Here's what changed recently.

### Recording: no more silently missing top-processes sidecar
- If the process collector can't start, the **Top processes** field in the
  record dialog is now disabled, with a "process collector unavailable; sidecar
  off" caption — the sidecar can no longer be requested silently.
- The app logs when a session asked for top processes but the collector isn't
  available, mirroring the CLI's diagnostic.
