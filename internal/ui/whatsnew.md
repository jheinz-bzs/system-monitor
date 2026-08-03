# What's New

## System Monitor

Thanks for updating. Here's what changed recently.

### Session recordings: compact and process snapshots
- Recording a session can now write **compressed** output — the same CSV data
  in a `.csv.gz` file that opens anywhere gzip is supported.
- A new **top processes sidecar** records the busiest processes alongside the
  metrics, so a spike in the charts can be traced to what caused it.
- Both are opt-in from the recording command line; default recordings are
  unchanged.
