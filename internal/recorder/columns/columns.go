// Package columns is the one authoritative home for the tracking-session CSV
// schema (BZS253-77): the column header strings the recorder writes and the
// Recordings tab matches on, the default session filename, the builder that
// adapts live collectors into the recorder's column set, and the session-option
// assembly (compact output, top-processes sidecar) both entry points share.
//
// It exists so the GUI composition root (internal/ui.Run) and the headless
// recording binary (cmd/system-monitor-record) write byte-identical schemas —
// a recording made on a server opens unchanged in the desktop app. It imports
// monitor and series, which is why it lives beside internal/recorder rather
// than inside it: the recorder package itself stays dependency-free (ADR-012).
package columns

import (
	"io"
	"log"
	"os"
	"strings"
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
	filePrefix          = "tracking-"
	fileExt             = ".csv"
	fileStamp           = "20060102-150405"
	compactExt          = ".gz"
	processesFileSuffix = ".processes.csv"
)

// FileName is the default name for a session recorded at t — offered in the
// GUI save dialog and used by the headless binary when --out is omitted.
func FileName(t time.Time) string {
	return filePrefix + t.Format(fileStamp) + fileExt
}

// CompactFilePath is the gzip-compacted path for a session written to path: the
// .gz suffix is appended unless already present, so tracking-… .csv becomes
// tracking-… .csv.gz.
func CompactFilePath(path string) string {
	if strings.HasSuffix(path, compactExt) {
		return path
	}
	return path + compactExt
}

// ProcessesFilePath is the top-processes sidecar path that belongs to a session
// written to path: the recording extension is swapped for the sidecar suffix, so
// tracking-… .csv (or .csv.gz) records beside tracking-… .processes.csv.
func ProcessesFilePath(path string) string {
	base := strings.TrimSuffix(path, compactExt)
	base = strings.TrimSuffix(base, fileExt)
	return base + processesFileSuffix
}

// ValidPath reports whether a session path is usable: non-empty after trimming,
// so a modal confirm with a cleared location doesn't try to create a nameless
// file. Relative paths are allowed — a bare filename writes into the working
// directory, matching the headless binary's --out default.
func ValidPath(path string) bool {
	return strings.TrimSpace(path) != ""
}

// SessionOptions assembles the recorder's format options for a session spec
// written to path — the one translation behind both the GUI record modal and
// the headless binary's flags. Compact selects the gzip'd .csv.gz output; a
// TopN > 0 spec adds a top-processes sidecar (beside path) sampled from the live
// collector every tick. procs feeds the snapshot; a nil collector or TopN 0
// drops the sidecar. The caller passes the final output path — CompactFilePath
// already applied when Compact is set — so the sidecar name derives from the
// same file the caller will create.
func SessionOptions(spec recorder.OptionsSpec, procs *monitor.ProcessCollector, path string) []recorder.Option {
	return recorder.Options(spec, topProcesses(spec, procs), openSidecar(ProcessesFilePath(path)))
}

// SidecarArmed reports whether a session started from spec will write the
// top-processes sidecar: the spec asked for it (TopN > 0) and a live process
// collector feeds the snapshots. SessionOptions drops the sidecar when this is
// false, so a composition root can warn the user — the headless binary's
// "--processes %d ignored" diagnostic — instead of silently missing the file.
func SidecarArmed(spec recorder.OptionsSpec, procs *monitor.ProcessCollector) bool {
	return spec.TopN > 0 && procs != nil
}

// topProcesses adapts the process collector into the recorder's snapshot seam,
// returning the spec.TopN busiest-by-CPU processes with only the columns the
// sidecar writes. Nil when SidecarArmed is false, so recorder.Options drops the
// sidecar rather than recording an empty file. Reads the collector's latest
// cached snapshot per call; the poller owns the expensive enumeration.
func topProcesses(spec recorder.OptionsSpec, procs *monitor.ProcessCollector) recorder.ProcessSnapshot {
	if !SidecarArmed(spec, procs) {
		return nil
	}
	return func() []recorder.ProcessSample {
		ps := procs.Processes()
		samples := make([]recorder.ProcessSample, len(ps))
		for i, p := range ps {
			samples[i] = recorder.ProcessSample{
				PID:  p.PID,
				Name: p.Name,
				CPU:  p.CPUPercent,
				RSS:  p.MemoryBytes,
			}
		}
		return recorder.TopSamples(samples, spec.TopN)
	}
}

// openSidecar opens a top-processes sidecar path on the recorder's first
// snapshot, returning nil on failure so the recorder skips the snapshot rather
// than crashing — the sidecar is ancillary, never allowed to kill a session.
func openSidecar(path string) func() io.WriteCloser {
	return func() io.WriteCloser {
		f, err := os.Create(path)
		if err != nil {
			log.Printf("cannot open sidecar %s: %v", path, err)
			return nil
		}
		return f
	}
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
