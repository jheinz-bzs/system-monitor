// Package metrics holds metric history in in-memory ring buffers.
//
// The app keeps roughly the last ten minutes of samples at one-second
// resolution. There is no database or file I/O for metrics — when the process
// exits, history is gone.
//
// Implementation is added in later stories; this file establishes the package.
package metrics
