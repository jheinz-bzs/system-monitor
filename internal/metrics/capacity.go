package metrics

// HistoryCapacity is the number of samples every metric ring buffer retains:
// ~1 minute of history at 1-second resolution. All collectors size their
// buffers from this single constant so the retention window stays identical
// across metrics.
const HistoryCapacity = 60
