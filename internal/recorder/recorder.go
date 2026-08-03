// Package recorder implements the opt-in session tracking mode (BZS253-77):
// while a session is active it appends one CSV row per poll tick, capturing a
// longer window than the in-memory ring buffers hold so a spike can be inspected
// after the fact.
//
// It sits behind the poller's OnTick seam and its data path is deliberately
// dependency-free: each column is read through a plain func() float64 supplied
// by the composition root, and rows are written through an io.WriteCloser the
// caller hands it. So it imports no UI, no monitor, and no Fyne, and is
// unit-testable without a running app. Recording is session scope — it defaults
// off and is armed only by an explicit Start, disarmed by Stop (ADR-012).
//
// Two opt-in options extend a session without changing its default shape:
// Compact() writes the same rows gzip-compressed (.csv.gz), flushing the deflate
// stream per row so a crash keeps the file decompressible through the last row;
// WithProcessSnapshots writes a top-processes sidecar next to the session so a
// spike can be attributed to the processes that caused it. Both are construction
// options fixed for the recorder's lifetime; a session still starts and stops
// identically.
package recorder

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"sync"
	"time"
)

// headerTimestamp is the fixed first column; every metric column follows it.
const headerTimestamp = "timestamp"

// timestampLayout formats each row's clock in a spreadsheet- and grep-friendly
// ISO-8601 form (RFC3339, second resolution — the poll cadence).
const timestampLayout = time.RFC3339

// A sample renders with 'f' and full precision (-1): whole byte counts stay
// integral ("1048576") while fractional percentages still show ("42.5"), with
// no exponent form to trip up a spreadsheet import.
const (
	sampleFmt  byte = 'f'
	samplePrec      = -1
	sampleBits      = 64
)

// gzipMagic is the two-byte header every gzip stream begins with. Read peeks
// for it so a compact recording is opened transparently, without a separate
// entry point.
var gzipMagic = []byte{0x1f, 0x8b}

// Sidecar column headers, in row order after the recorder's fixed timestamp
// column. Units live in the names so a bare sidecar is self-describing,
// mirroring the metric CSV convention.
const (
	sidecarPID  = "pid"
	sidecarName = "name"
	sidecarCPU  = "cpu_pct"
	sidecarRSS  = "rss_bytes"
)

// ErrAlreadyRecording is returned by Start when a session is already active, so
// the caller still owns (and can close) the writer it passed rather than leaking
// it under a silently ignored second start.
var ErrAlreadyRecording = errors.New("recorder: already recording")

// Column is one metric written per row: a stable CSV header and a reader that
// returns its current value. The reader is a bare func so the recorder depends
// on neither the series seam nor any collector — the composition root adapts a
// series.Source (or a static total) into one.
type Column struct {
	Header string
	Read   func() float64
}

// ProcessSample is one process in a top-processes sidecar snapshot.
type ProcessSample struct {
	PID  int32
	Name string
	CPU  float64 // machine-wide percent, matching the cpu_pct metric column
	RSS  uint64  // resident set size in bytes
}

// ProcessSnapshot returns the current top processes to record, busiest first.
// It is the process analog of Column.Read: a bare func the composition root
// adapts from a collector, keeping the recorder dependency-free.
type ProcessSnapshot func() []ProcessSample

// Option configures a Recorder at construction. A recorder's options are fixed
// for its lifetime; Start and Stop only arm and disarm sessions over the same
// format.
type Option func(*Recorder)

// Compact makes sessions write gzip-compressed CSV (.csv.gz) instead of plain
// CSV: the same rows, decompressible by any gzip tool. The deflate stream
// flushes on every row, so a crash mid-session leaves the file readable through
// the last flushed row — the same crash-safety the plain CSV has. It is opt-in;
// the default stays plain CSV so spreadsheet tooling keeps working.
func Compact() Option {
	return func(r *Recorder) { r.compact = true }
}

// WithProcessSnapshots records a top-processes sidecar for spike attribution:
// every every-th Tick (1 = every tick) the recorder writes one CSV row per
// process returned by snap to a file opened via open on the first snapshot, so
// nothing is opened for a session that ends before a snapshot tick. Rows share
// the metric row's timestamp, so the sidecar and the session file join on the
// timestamp column; the main CSV's schema is untouched.
func WithProcessSnapshots(snap ProcessSnapshot, every int, open func() io.WriteCloser) Option {
	return func(r *Recorder) {
		r.snap = snap
		r.snapEvery = every
		r.openSnap = open
	}
}

// Recorder appends one CSV row per Tick while a session is active. It is inert
// until Start and after Stop; a mutex guards the active writer so the UI's
// start/stop taps can't race the poller goroutine driving Tick.
type Recorder struct {
	columns []Column
	now     func() time.Time // seam for tests; time.Now in production

	// Construction-time format options (immutable once New returns).
	compact   bool
	snap      ProcessSnapshot
	snapEvery int
	openSnap  func() io.WriteCloser

	// Session state, guarded by mu.
	mu         sync.Mutex
	file       io.WriteCloser
	gz         *gzip.Writer // non-nil in a compact session
	writer     *csv.Writer
	tick       int
	snapFile   io.WriteCloser
	snapWriter *csv.Writer
}

// New builds a Recorder over the given columns and options. The column set is
// fixed for the process; a session only starts and stops the writing, never
// changes columns. No options means the default plain-CSV session.
func New(columns []Column, opts ...Option) *Recorder {
	r := &Recorder{columns: columns, now: time.Now}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Start arms recording, writing the header row to w and flushing it so the file
// is a valid, openable CSV the instant a session begins; a compact recorder
// writes the row through a gzip stream instead. It adopts w only on success; if
// the header can't be written the caller keeps ownership of w. Starting an
// already-active session returns ErrAlreadyRecording.
func (r *Recorder) Start(w io.WriteCloser) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.writer != nil {
		return ErrAlreadyRecording
	}
	var gz *gzip.Writer
	var out io.Writer = w
	if r.compact {
		gz = gzip.NewWriter(w)
		out = gz
	}
	cw := csv.NewWriter(out)
	if err := cw.Write(r.header()); err != nil {
		return fmt.Errorf("recorder: write header: %w", err)
	}
	r.writer, r.gz = cw, gz
	if err := r.flush(); err != nil {
		r.writer, r.gz = nil, nil
		return fmt.Errorf("recorder: flush header: %w", err)
	}
	r.file = w
	return nil
}

// Tick appends one row — the current timestamp plus each column's latest value —
// when a session is active, flushing per row so a crash mid-session keeps every
// row already written. It is a no-op when inert. Runs on the poller goroutine; a
// write error is logged and the session left open so a transient failure doesn't
// drop the whole recording. A recorder armed with WithProcessSnapshots writes a
// top-processes snapshot on the configured tick cadence, sharing the row's
// timestamp so the two files join.
func (r *Recorder) Tick() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.writer == nil {
		return
	}
	ts := r.now()
	row := make([]string, 0, len(r.columns)+1)
	row = append(row, ts.Format(timestampLayout))
	for _, c := range r.columns {
		row = append(row, strconv.FormatFloat(c.Read(), sampleFmt, samplePrec, sampleBits))
	}
	if err := r.writer.Write(row); err != nil {
		slog.Error("recorder write row", "err", err)
		return
	}
	if err := r.flush(); err != nil {
		slog.Error("recorder flush row", "err", err)
	}
	r.tick++
	if r.snapEvery > 0 && r.tick%r.snapEvery == 0 {
		r.snapshot(ts)
	}
}

// flush pushes the csv writer's buffer into the session file and, in a compact
// session, forces the gzip deflate stream out too — so a crash after a row
// leaves the file decompressible through it.
func (r *Recorder) flush() error {
	r.writer.Flush()
	if err := r.writer.Error(); err != nil {
		return err
	}
	if r.gz != nil {
		return r.gz.Flush()
	}
	return nil
}

// snapshot writes one row per process from the current sample to the sidecar,
// opening it on the first snapshot and flushing per snapshot so a crash keeps
// every snapshot already written. Rows share the metric row's timestamp, so the
// two files join on it. Failures are logged and the session left open, matching
// the main CSV's degrade-quietly behavior.
func (r *Recorder) snapshot(ts time.Time) {
	if r.snapWriter == nil {
		w := r.openSnap()
		if w == nil {
			slog.Error("recorder sidecar open failed")
			return
		}
		sw := csv.NewWriter(w)
		if err := sw.Write(sidecarHeader()); err != nil {
			w.Close()
			slog.Error("recorder sidecar header", "err", err)
			return
		}
		sw.Flush()
		if err := sw.Error(); err != nil {
			w.Close()
			slog.Error("recorder sidecar header", "err", err)
			return
		}
		r.snapFile, r.snapWriter = w, sw
	}
	for _, p := range r.snap() {
		if err := r.snapWriter.Write(snapshotRow(ts, p)); err != nil {
			slog.Error("recorder sidecar row", "err", err)
			return
		}
	}
	r.snapWriter.Flush()
	if err := r.snapWriter.Error(); err != nil {
		slog.Error("recorder sidecar flush", "err", err)
	}
}

// Stop flushes and closes the session file and disarms recording, so the output
// is complete and openable. In a compact session it closes the gzip stream
// first, which writes the footer that makes the file a valid .csv.gz. A
// top-processes sidecar, when one was opened, is flushed and closed too.
// Stopping when inactive is a no-op.
func (r *Recorder) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.writer == nil {
		return nil
	}
	r.writer.Flush()
	flushErr := r.writer.Error()
	if flushErr == nil && r.gz != nil {
		flushErr = r.gz.Close()
	}
	closeErr := r.file.Close()

	var sidecarErr error
	if r.snapWriter != nil {
		r.snapWriter.Flush()
		sidecarErr = r.snapWriter.Error()
		if closeSnapErr := r.snapFile.Close(); sidecarErr == nil {
			sidecarErr = closeSnapErr
		}
	}

	r.writer, r.file, r.gz = nil, nil, nil
	r.snapWriter, r.snapFile = nil, nil
	r.tick = 0
	if flushErr != nil {
		return fmt.Errorf("recorder: flush: %w", flushErr)
	}
	if closeErr != nil {
		return fmt.Errorf("recorder: close: %w", closeErr)
	}
	if sidecarErr != nil {
		return fmt.Errorf("recorder: sidecar close: %w", sidecarErr)
	}
	return nil
}

// Recording reports whether a session is currently active — the UI reads it to
// paint the record control and to choose start vs stop on tap.
func (r *Recorder) Recording() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.writer != nil
}

// header is the timestamp column plus each metric column header, in column order.
func (r *Recorder) header() []string {
	row := make([]string, 0, len(r.columns)+1)
	row = append(row, headerTimestamp)
	for _, c := range r.columns {
		row = append(row, c.Header)
	}
	return row
}

// sidecarHeader is the top-processes sidecar's column order: the metric
// timestamp column plus the four process columns.
func sidecarHeader() []string {
	return []string{headerTimestamp, sidecarPID, sidecarName, sidecarCPU, sidecarRSS}
}

// snapshotRow renders one process as a sidecar CSV row, reusing the metric
// row's timestamp and sample formatting so the two files join and format alike.
func snapshotRow(ts time.Time, p ProcessSample) []string {
	return []string{
		ts.Format(timestampLayout),
		strconv.FormatInt(int64(p.PID), 10),
		p.Name,
		strconv.FormatFloat(p.CPU, sampleFmt, samplePrec, sampleBits),
		strconv.FormatUint(p.RSS, 10),
	}
}

// Recording is a parsed session CSV: the timestamp column plus each metric
// column's samples, aligned by row. It is the inverse of what Start/Tick write —
// Read round-trips anything this package produced — and lets a viewer replay a
// recording without re-implementing the format.
type Recording struct {
	Timestamps []time.Time
	Columns    []string    // metric headers in file order (timestamp excluded)
	Series     [][]float64 // one slice per Column, each len == len(Timestamps)
}

// Read parses a tracking-session recording — plain CSV or, transparently, a
// .csv.gz compact recording (detected by its gzip header): a header row
// (timestamp + metric columns) followed by one row per tick. An empty file, a
// header that isn't ours, or an unparseable timestamp/sample is an error rather
// than silently-zeroed data, so a corrupt file surfaces instead of plotting
// garbage. csv.Reader enforces a uniform field count, so every data row lines up
// with the header.
func Read(r io.Reader) (*Recording, error) {
	br := bufio.NewReader(r)
	if isGzip(br) {
		gz, err := gzip.NewReader(br)
		if err != nil {
			return nil, fmt.Errorf("recorder: open gzip: %w", err)
		}
		defer gz.Close()
		br = bufio.NewReader(gz)
	}
	return readCSV(br)
}

// isGzip reports whether the stream begins with the gzip magic bytes, so Read
// can open both plain and compact recordings without a separate entry point.
func isGzip(r *bufio.Reader) bool {
	magic, err := r.Peek(len(gzipMagic))
	return err == nil && bytes.Equal(magic, gzipMagic)
}

// readCSV parses the CSV body shared by plain and compact recordings.
func readCSV(r io.Reader) (*Recording, error) {
	rows, err := csv.NewReader(r).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("recorder: read csv: %w", err)
	}
	if len(rows) == 0 {
		return nil, errors.New("recorder: empty recording")
	}
	header := rows[0]
	if len(header) < 2 || header[0] != headerTimestamp {
		return nil, fmt.Errorf("recorder: unexpected header %v", header)
	}
	cols := header[1:]
	rec := &Recording{Columns: cols, Series: make([][]float64, len(cols))}
	for i, row := range rows[1:] {
		ts, err := time.Parse(timestampLayout, row[0])
		if err != nil {
			return nil, fmt.Errorf("recorder: row %d timestamp: %w", i+1, err)
		}
		rec.Timestamps = append(rec.Timestamps, ts)
		for c := range cols {
			v, err := strconv.ParseFloat(row[c+1], sampleBits)
			if err != nil {
				return nil, fmt.Errorf("recorder: row %d %s: %w", i+1, cols[c], err)
			}
			rec.Series[c] = append(rec.Series[c], v)
		}
	}
	return rec, nil
}
