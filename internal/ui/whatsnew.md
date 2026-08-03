# What's New

## System Monitor

Thanks for updating. Here's what changed recently.

### Recording: a dialog with real options
- Tapping the record button now opens a dialog before a session starts:
  - **Compact output** — gzip-compressed `.csv.gz` instead of plain CSV.
  - **Top processes** — record the N busiest-by-CPU processes each tick to a
    sidecar file (0 = off).
  - **Save location** — see and edit exactly where the file will be written,
    or Browse… to pick a folder; the default filename is pre-filled.
- Cancel leaves recording idle, and tapping the toggle during an active
  session still stops it right away.
