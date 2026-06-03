// Package metrics holds metric history in in-memory ring buffers.
//
// The app keeps roughly the last minute of samples at one-second resolution
// (see HistoryCapacity). There is no database or file I/O for metrics — when
// the process exits, history is gone.
//
// Implementation is added in later stories; this file establishes the package.
package metrics
