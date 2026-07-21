// Package columns is the one authoritative home for the tracking-session CSV
// schema (BZS253-77): the column header strings the recorder writes and the
// Recordings tab matches on, the default session filename, and the builder that
// adapts live collectors into the recorder's column set.
//
// It exists so the GUI composition root (internal/ui.Run) and the headless
// recording binary (cmd/system-monitor-record) write byte-identical schemas —
// a recording made on a server opens unchanged in the desktop app. It imports
// monitor and series, which is why it lives beside internal/recorder rather
// than inside it: the recorder package itself stays dependency-free (ADR-012).
package columns

import (
	"time"

	"github.com/josephheinz/system-monitor/internal/monitor"
	"github.com/josephheinz/system-monitor/internal/recorder"
	"github.com/josephheinz/system-monitor/internal/series"
)

// CSV column headers, in row order after the recorder's fixed timestamp column.
// They are the file's stable schema — the Recordings tab matches columns by
// these exact strings — so changing one orphans every previously saved
// recording. Units are in the name so a bare CSV is self-describing.
const (
	CPUPct    = "cpu_pct"
	MemUsed   = "mem_used_bytes"
	MemTotal  = "mem_total_bytes"
	SwapUsed  = "swap_used_bytes"
	NetRx     = "net_rx_bytes_per_s"
	NetTx     = "net_tx_bytes_per_s"
	DiskRead  = "disk_read_bytes_per_s"
	DiskWrite = "disk_write_bytes_per_s"
	ProcCount = "proc_count"
)

// Default session filename: a session stamp so successive recordings don't
// collide. The stamp is a Go reference-time layout, not a magic number.
const (
	filePrefix = "tracking-"
	fileExt    = ".csv"
	fileStamp  = "20060102-150405"
)

// FileName is the default name for a session recorded at t — offered in the
// GUI save dialog and used by the headless binary when --out is omitted.
func FileName(t time.Time) string {
	return filePrefix + t.Format(fileStamp) + fileExt
}

// Build adapts the live collectors into the tracking-mode column set, in the
// fixed schema order. A nil collector (one that failed to start) contributes
// columns that record 0, so the CSV keeps a stable header regardless of which
// collectors came up. rx/tx map to download/upload; disk rates are the I/O
// series, not volume usage.
func Build(cpu *monitor.CPUCollector, mem *monitor.MemoryCollector, disk *monitor.DiskCollector, net *monitor.NetworkCollector, procs *monitor.ProcessCollector) []recorder.Column {
	var cpuPct, memUsed, swapUsed, netRx, netTx, diskRead, diskWrite, procCount series.Source
	var memTotal uint64
	if cpu != nil {
		cpuPct = series.SourceFunc(cpu.Overall)
	}
	if mem != nil {
		memUsed = series.SourceOf(mem.Used)
		swapUsed = series.SourceOf(mem.SwapUsed)
		memTotal = mem.Total()
	}
	if net != nil {
		netRx = series.SourceOf(net.DownloadRate)
		netTx = series.SourceOf(net.UploadRate)
	}
	if disk != nil {
		diskRead = series.SourceOf(disk.ReadRate)
		diskWrite = series.SourceOf(disk.WriteRate)
	}
	if procs != nil {
		procCount = series.SourceOf(procs.Count)
	}
	return []recorder.Column{
		{Header: CPUPct, Read: latest(cpuPct)},
		{Header: MemUsed, Read: latest(memUsed)},
		{Header: MemTotal, Read: constant(memTotal)},
		{Header: SwapUsed, Read: latest(swapUsed)},
		{Header: NetRx, Read: latest(netRx)},
		{Header: NetTx, Read: latest(netTx)},
		{Header: DiskRead, Read: latest(diskRead)},
		{Header: DiskWrite, Read: latest(diskWrite)},
		{Header: ProcCount, Read: latest(procCount)},
	}
}

// latest reads a source's newest sample, 0 when the source is unwired or empty.
func latest(s series.Source) func() float64 {
	return func() float64 {
		if s == nil {
			return 0
		}
		vals := s.Values()
		if len(vals) == 0 {
			return 0
		}
		return vals[len(vals)-1]
	}
}

// constant records a fixed value (e.g. total physical RAM) every tick.
func constant(v uint64) func() float64 {
	return func() float64 { return float64(v) }
}
