# What's New

## System Monitor

Thanks for updating. Here's what changed recently.

### Linux: the Disk treemap now matches reality
- The directory treemap no longer absorbs separate filesystems mounted under a
  volume — `/` shows only `/`'s storage, not `/proc`, `/sys`, or a separate
  `/home` — so volume totals agree with the Volumes panel.
- File-backed mounts (like a container's `/etc/hostname`) no longer appear as
  phantom volumes with empty treemaps.
- Unreadable directories (other users' homes, `/root`) are now counted and
  reported instead of silently skipped, so a partial crawl is visible rather
  than looking complete.
