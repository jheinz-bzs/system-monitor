package columns

import (
	"bytes"
	"testing"
	"time"

	"github.com/josephheinz/system-monitor/internal/recorder"
)

// schemaHeaders is the published column order — the contract with the
// Recordings tab and every previously saved CSV.
var schemaHeaders = []string{
	CPUPct, MemUsed, MemTotal, SwapUsed,
	NetRx, NetTx, DiskRead, DiskWrite, ProcCount,
}

func TestBuildSchemaOrder(t *testing.T) {
	cols := Build(nil, nil, nil, nil, nil)
	if len(cols) != len(schemaHeaders) {
		t.Fatalf("got %d columns, want %d", len(cols), len(schemaHeaders))
	}
	for i, c := range cols {
		if c.Header != schemaHeaders[i] {
			t.Errorf("column %d header = %q, want %q", i, c.Header, schemaHeaders[i])
		}
	}
}

func TestBuildNilCollectorsReadZero(t *testing.T) {
	for _, c := range Build(nil, nil, nil, nil, nil) {
		if got := c.Read(); got != 0 {
			t.Errorf("%s read %v with nil collectors, want 0", c.Header, got)
		}
	}
}

// nopCloser adapts a bytes.Buffer to the io.WriteCloser the recorder adopts.
type nopCloser struct{ *bytes.Buffer }

func (nopCloser) Close() error { return nil }

func TestBuildRoundTripsThroughRecorder(t *testing.T) {
	rec := recorder.New(Build(nil, nil, nil, nil, nil))
	var buf bytes.Buffer
	if err := rec.Start(nopCloser{&buf}); err != nil {
		t.Fatalf("start: %v", err)
	}
	rec.Tick()
	if err := rec.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}

	got, err := recorder.Read(&buf)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if len(got.Columns) != len(schemaHeaders) {
		t.Fatalf("got %d columns, want %d", len(got.Columns), len(schemaHeaders))
	}
	for i, h := range got.Columns {
		if h != schemaHeaders[i] {
			t.Errorf("column %d = %q, want %q", i, h, schemaHeaders[i])
		}
	}
	if len(got.Timestamps) != 1 {
		t.Fatalf("got %d rows, want 1", len(got.Timestamps))
	}
}

func TestFileName(t *testing.T) {
	at := time.Date(2026, 7, 13, 9, 30, 5, 0, time.UTC)
	if got, want := FileName(at), "tracking-20260713-093005.csv"; got != want {
		t.Errorf("FileName = %q, want %q", got, want)
	}
}

func TestCompactFilePath(t *testing.T) {
	at := time.Date(2026, 7, 13, 9, 30, 5, 0, time.UTC)
	if got, want := CompactFilePath(FileName(at)), "tracking-20260713-093005.csv.gz"; got != want {
		t.Errorf("CompactFilePath = %q, want %q", got, want)
	}
	if got, want := CompactFilePath("foo.csv"), "foo.csv.gz"; got != want {
		t.Errorf("CompactFilePath = %q, want %q", got, want)
	}
	// Idempotent: a path already compacted is returned unchanged.
	if got, want := CompactFilePath("foo.csv.gz"), "foo.csv.gz"; got != want {
		t.Errorf("CompactFilePath = %q, want %q", got, want)
	}
}

func TestProcessesFilePath(t *testing.T) {
	at := time.Date(2026, 7, 13, 9, 30, 5, 0, time.UTC)
	if got, want := ProcessesFilePath(FileName(at)), "tracking-20260713-093005.processes.csv"; got != want {
		t.Errorf("ProcessesFilePath = %q, want %q", got, want)
	}
	// A compact session's sidecar drops the compression extension too, so both
	// sessions write the same sidecar name.
	if got, want := ProcessesFilePath("tracking-20260713-093005.csv.gz"), "tracking-20260713-093005.processes.csv"; got != want {
		t.Errorf("ProcessesFilePath = %q, want %q", got, want)
	}
}
