// Package monitor collects live system metrics (CPU, memory, disk, network,
// processes, ports, connections) using gopsutil.
//
// Collectors here feed the in-memory ring buffers in package metrics; there is
// no persistence layer. Process IDs are treated as first-class identifiers so
// that cross-tab navigation can be wired up cleanly.
//
// Implementation is added in later stories; this file establishes the package.
package monitor
