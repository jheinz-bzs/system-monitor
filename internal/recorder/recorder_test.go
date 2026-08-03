package recorder

import (
	"bytes"
	"compress/gzip"
	"encoding/csv"
	"io"
	"testing"
	"time"
)

// nopCloser adapts a bytes.Buffer to io.WriteCloser so a test can inspect the
// written CSV without touching the filesystem — the whole point of the recorder
// taking an io.WriteCloser rather than a path.
type nopCloser struct {
	*bytes.Buffer
	closed bool
}

func (c *nopCloser) Close() error { c.closed = true; return nil }

// testColumns is a stable two-column set with deterministic readers so row
// contents are predictable.
func testColumns() []Column {
	return []Column{
		{Header: "cpu_pct", Read: func() float64 { return 42.5 }},
		{Header: "mem_used_bytes", Read: func() float64 { return 1048576 }},
	}
}

// fixedNow returns a constant clock so the timestamp column is deterministic.
func fixedNow() func() time.Time {
	t := time.Date(2026, 7, 8, 12, 0, 0, 0, time.UTC)
	return func() time.Time { return t }
}

func TestTickWritesOneRowPerTick(t *testing.T) {
	rec := New(testColumns())
	rec.now = fixedNow()
	buf := &nopCloser{Buffer: &bytes.Buffer{}}

	if err := rec.Start(buf); err != nil {
		t.Fatalf("Start: %v", err)
	}
	const ticks = 3
	for range ticks {
		rec.Tick()
	}
	if err := rec.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	rows, err := csv.NewReader(bytes.NewReader(buf.Bytes())).ReadAll()
	if err != nil {
		t.Fatalf("output is not valid CSV: %v", err)
	}
	// Header + one row per tick.
	if want := ticks + 1; len(rows) != want {
		t.Fatalf("row count = %d, want %d", len(rows), want)
	}
	wantHeader := []string{"timestamp", "cpu_pct", "mem_used_bytes"}
	if got := rows[0]; !equal(got, wantHeader) {
		t.Fatalf("header = %v, want %v", got, wantHeader)
	}
	wantRow := []string{"2026-07-08T12:00:00Z", "42.5", "1048576"}
	for i, r := range rows[1:] {
		if !equal(r, wantRow) {
			t.Fatalf("row %d = %v, want %v", i, r, wantRow)
		}
	}
	if !buf.closed {
		t.Fatal("Stop did not close the file")
	}
}

func TestInertUntilStartedAndAfterStopped(t *testing.T) {
	rec := New(testColumns())
	rec.now = fixedNow()

	// Before Start: Tick writes nothing, Recording is false.
	if rec.Recording() {
		t.Fatal("Recording() true before Start")
	}
	rec.Tick() // must not panic on a nil writer

	buf := &nopCloser{Buffer: &bytes.Buffer{}}
	if err := rec.Start(buf); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !rec.Recording() {
		t.Fatal("Recording() false while active")
	}
	rec.Tick()
	if err := rec.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	// After Stop: Tick must not append to the closed session.
	before := buf.Len()
	rec.Tick()
	if buf.Len() != before {
		t.Fatal("Tick wrote after Stop")
	}
	if rec.Recording() {
		t.Fatal("Recording() true after Stop")
	}
}

func TestStartTwiceReturnsError(t *testing.T) {
	rec := New(testColumns())
	first := &nopCloser{Buffer: &bytes.Buffer{}}
	if err := rec.Start(first); err != nil {
		t.Fatalf("first Start: %v", err)
	}
	second := &nopCloser{Buffer: &bytes.Buffer{}}
	if err := rec.Start(second); err != ErrAlreadyRecording {
		t.Fatalf("second Start err = %v, want ErrAlreadyRecording", err)
	}
	// The rejected writer stays the caller's to close — the recorder never
	// adopted it, so it wasn't closed.
	if second.closed {
		t.Fatal("rejected Start closed the caller's writer")
	}
}

func TestReadRoundTrip(t *testing.T) {
	rec := New(testColumns())
	rec.now = fixedNow()
	buf := &nopCloser{Buffer: &bytes.Buffer{}}
	if err := rec.Start(buf); err != nil {
		t.Fatalf("Start: %v", err)
	}
	const ticks = 3
	for range ticks {
		rec.Tick()
	}
	if err := rec.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	got, err := Read(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if want := []string{"cpu_pct", "mem_used_bytes"}; !equal(got.Columns, want) {
		t.Fatalf("columns = %v, want %v", got.Columns, want)
	}
	if len(got.Timestamps) != ticks {
		t.Fatalf("timestamps = %d, want %d", len(got.Timestamps), ticks)
	}
	for c, want := range []float64{42.5, 1048576} {
		if len(got.Series[c]) != ticks {
			t.Fatalf("series[%d] len = %d, want %d", c, len(got.Series[c]), ticks)
		}
		if got.Series[c][0] != want {
			t.Fatalf("series[%d][0] = %v, want %v", c, got.Series[c][0], want)
		}
	}
}

func TestReadRejectsForeignHeader(t *testing.T) {
	if _, err := Read(bytes.NewReader([]byte("when,cpu\n1,2\n"))); err == nil {
		t.Fatal("Read accepted a header without the timestamp column")
	}
	if _, err := Read(bytes.NewReader(nil)); err == nil {
		t.Fatal("Read accepted an empty file")
	}
}

// recordCSV runs a three-tick session and returns the written bytes, so tests
// can compare the plain and compact outputs of identical sessions.
func recordCSV(t *testing.T, compact bool) []byte {
	t.Helper()
	rec := New(testColumns())
	if compact {
		rec = New(testColumns(), Compact())
	}
	rec.now = fixedNow()
	buf := &nopCloser{Buffer: &bytes.Buffer{}}
	if err := rec.Start(buf); err != nil {
		t.Fatalf("Start: %v", err)
	}
	const ticks = 3
	for range ticks {
		rec.Tick()
	}
	if err := rec.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	return buf.Bytes()
}

func TestDefaultOutputIsPlainCSV(t *testing.T) {
	out := recordCSV(t, false)
	if bytes.HasPrefix(out, gzipMagic) {
		t.Fatal("default session output is gzip-compressed; expected plain CSV")
	}
}

func TestCompactOutputDecompressesToSameRows(t *testing.T) {
	plain := recordCSV(t, false)
	compact := recordCSV(t, true)

	if !bytes.HasPrefix(compact, gzipMagic) {
		t.Fatal("compact output is not a gzip stream")
	}

	// The decompressed stream is byte-identical to the plain-CSV output, so
	// existing tooling only needs gzip, not a new format.
	gz, err := gzip.NewReader(bytes.NewReader(compact))
	if err != nil {
		t.Fatalf("gzip.NewReader: %v", err)
	}
	got, err := io.ReadAll(gz)
	if err != nil {
		t.Fatalf("decompress: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip reader: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatal("decompressed compact output differs from the plain CSV output")
	}
}

func TestCompactRoundTrip(t *testing.T) {
	rec := New(testColumns(), Compact())
	rec.now = fixedNow()
	buf := &nopCloser{Buffer: &bytes.Buffer{}}
	if err := rec.Start(buf); err != nil {
		t.Fatalf("Start: %v", err)
	}
	const ticks = 3
	for range ticks {
		rec.Tick()
	}
	if err := rec.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	got, err := Read(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Read compact: %v", err)
	}
	if want := []string{"cpu_pct", "mem_used_bytes"}; !equal(got.Columns, want) {
		t.Fatalf("columns = %v, want %v", got.Columns, want)
	}
	if len(got.Timestamps) != ticks {
		t.Fatalf("timestamps = %d, want %d", len(got.Timestamps), ticks)
	}
	if got.Series[0][0] != 42.5 {
		t.Fatalf("series[0][0] = %v, want 42.5", got.Series[0][0])
	}
	if got.Series[1][0] != 1048576 {
		t.Fatalf("series[1][0] = %v, want 1048576", got.Series[1][0])
	}
}

// testSnapshot returns a deterministic two-process sample.
func testSnapshot() []ProcessSample {
	return []ProcessSample{
		{PID: 42, Name: "bash", CPU: 12.5, RSS: 2048},
		{PID: 7, Name: "init", CPU: 1.25, RSS: 4096},
	}
}

func TestProcessSnapshotsSidecar(t *testing.T) {
	sidecar := &nopCloser{Buffer: &bytes.Buffer{}}
	rec := New(testColumns(), WithProcessSnapshots(
		testSnapshot,
		1, // every tick
		func() io.WriteCloser { return sidecar },
	))
	rec.now = fixedNow()
	buf := &nopCloser{Buffer: &bytes.Buffer{}}
	if err := rec.Start(buf); err != nil {
		t.Fatalf("Start: %v", err)
	}
	const ticks = 3
	for range ticks {
		rec.Tick()
	}
	if err := rec.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if !sidecar.closed {
		t.Fatal("Stop did not close the sidecar")
	}

	rows, err := csv.NewReader(bytes.NewReader(sidecar.Bytes())).ReadAll()
	if err != nil {
		t.Fatalf("sidecar is not valid CSV: %v", err)
	}
	wantHeader := []string{"timestamp", "pid", "name", "cpu_pct", "rss_bytes"}
	if got := rows[0]; !equal(got, wantHeader) {
		t.Fatalf("sidecar header = %v, want %v", got, wantHeader)
	}
	if want := ticks * 2; len(rows[1:]) != want {
		t.Fatalf("sidecar rows = %d, want %d", len(rows[1:]), want)
	}
	// Every snapshot row shares the metric row's timestamp, so the sidecar and
	// the session join on it.
	for i, r := range rows[1:] {
		if r[0] != "2026-07-08T12:00:00Z" {
			t.Fatalf("row %d timestamp = %q, want the metric timestamp", i, r[0])
		}
	}
	wantRow := []string{"2026-07-08T12:00:00Z", "42", "bash", "12.5", "2048"}
	if !equal(rows[1], wantRow) {
		t.Fatalf("first snapshot row = %v, want %v", rows[1], wantRow)
	}
}

func TestProcessSnapshotsEveryNTicks(t *testing.T) {
	sidecar := &nopCloser{Buffer: &bytes.Buffer{}}
	rec := New(testColumns(), WithProcessSnapshots(
		func() []ProcessSample { return []ProcessSample{{PID: 1, Name: "one", CPU: 1, RSS: 1}} },
		2,
		func() io.WriteCloser { return sidecar },
	))
	rec.now = fixedNow()
	buf := &nopCloser{Buffer: &bytes.Buffer{}}
	if err := rec.Start(buf); err != nil {
		t.Fatalf("Start: %v", err)
	}
	const ticks = 4
	for range ticks {
		rec.Tick()
	}
	if err := rec.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	rows, err := csv.NewReader(bytes.NewReader(sidecar.Bytes())).ReadAll()
	if err != nil {
		t.Fatalf("sidecar is not valid CSV: %v", err)
	}
	const snapshots = 2 // at ticks 2 and 4 of 4, cadence 2
	if want := snapshots + 1; len(rows) != want {
		t.Fatalf("sidecar rows = %d, want %d (header + snapshots at ticks 2 and 4)", len(rows), want)
	}
}

func TestProcessSnapshotsLazyOpen(t *testing.T) {
	opened := false
	rec := New(testColumns(), WithProcessSnapshots(
		func() []ProcessSample { return nil },
		1,
		func() io.WriteCloser {
			opened = true
			return &nopCloser{Buffer: &bytes.Buffer{}}
		},
	))
	rec.now = fixedNow()
	buf := &nopCloser{Buffer: &bytes.Buffer{}}
	if err := rec.Start(buf); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := rec.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if opened {
		t.Fatal("sidecar opened even though no snapshot tick ran")
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
