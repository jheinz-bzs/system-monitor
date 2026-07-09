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
package recorder

import (
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

// Recorder appends one CSV row per Tick while a session is active. It is inert
// until Start and after Stop; a mutex guards the active writer so the UI's
// start/stop taps can't race the poller goroutine driving Tick.
type Recorder struct {
	columns []Column
	now     func() time.Time // seam for tests; time.Now in production

	mu     sync.Mutex
	file   io.WriteCloser
	writer *csv.Writer
}

// New builds a Recorder over the given columns. The column set is fixed for the
// process; a session only starts and stops the writing, never changes columns.
func New(columns ...Column) *Recorder {
	return &Recorder{columns: columns, now: time.Now}
}

// Start arms recording, writing the header row to w and flushing it so the file
// is a valid, openable CSV the instant a session begins. It adopts w only on
// success; if the header can't be written the caller keeps ownership of w.
// Starting an already-active session returns ErrAlreadyRecording.
func (r *Recorder) Start(w io.WriteCloser) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.writer != nil {
		return ErrAlreadyRecording
	}
	cw := csv.NewWriter(w)
	if err := cw.Write(r.header()); err != nil {
		return fmt.Errorf("recorder: write header: %w", err)
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		return fmt.Errorf("recorder: flush header: %w", err)
	}
	r.file, r.writer = w, cw
	return nil
}

// Tick appends one row — the current timestamp plus each column's latest value —
// when a session is active, flushing per row so a crash mid-session keeps every
// row already written. It is a no-op when inert. Runs on the poller goroutine; a
// write error is logged and the session left open so a transient failure doesn't
// drop the whole recording.
func (r *Recorder) Tick() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.writer == nil {
		return
	}
	row := make([]string, 0, len(r.columns)+1)
	row = append(row, r.now().Format(timestampLayout))
	for _, c := range r.columns {
		row = append(row, strconv.FormatFloat(c.Read(), sampleFmt, samplePrec, sampleBits))
	}
	if err := r.writer.Write(row); err != nil {
		slog.Error("recorder write row", "err", err)
		return
	}
	r.writer.Flush()
	if err := r.writer.Error(); err != nil {
		slog.Error("recorder flush row", "err", err)
	}
}

// Stop flushes and closes the session file and disarms recording, so the output
// is complete and openable. Stopping when inactive is a no-op.
func (r *Recorder) Stop() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.writer == nil {
		return nil
	}
	r.writer.Flush()
	flushErr := r.writer.Error()
	closeErr := r.file.Close()
	r.writer, r.file = nil, nil
	if flushErr != nil {
		return fmt.Errorf("recorder: flush: %w", flushErr)
	}
	if closeErr != nil {
		return fmt.Errorf("recorder: close: %w", closeErr)
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

// Recording is a parsed session CSV: the timestamp column plus each metric
// column's samples, aligned by row. It is the inverse of what Start/Tick write —
// Read round-trips anything this package produced — and lets a viewer replay a
// recording without re-implementing the format.
type Recording struct {
	Timestamps []time.Time
	Columns    []string    // metric headers in file order (timestamp excluded)
	Series     [][]float64 // one slice per Column, each len == len(Timestamps)
}

// Read parses a tracking-session CSV: a header row (timestamp + metric columns)
// followed by one row per tick. An empty file, a header that isn't ours, or an
// unparseable timestamp/sample is an error rather than silently-zeroed data, so
// a corrupt file surfaces instead of plotting garbage. csv.Reader enforces a
// uniform field count, so every data row lines up with the header.
func Read(r io.Reader) (*Recording, error) {
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
