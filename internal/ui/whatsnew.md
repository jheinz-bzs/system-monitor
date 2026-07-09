# What's New

## System Monitor

Thanks for updating. Here's what changed recently.

### Disk tab fixes
- Pseudo-volumes (snap loop images, RAM disks, overlay mounts) no longer show
  up as volumes — on Linux they could flood the **Disk** tab and crash the app,
  and on macOS stretch the window thousands of pixels wide.
- The volume selector and volumes list now scroll instead of widening the
  window when you have many volumes.
- The treemap's warm-start cache now persists for system installs (like the
  .deb), so the **Disk** tab isn't blank after every launch while it rescans.
