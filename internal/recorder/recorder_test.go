package recorder

import (
	"bytes"
	"encoding/csv"
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
	rec := New(testColumns()...)
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
	rec := New(testColumns()...)
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
	rec := New(testColumns()...)
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
