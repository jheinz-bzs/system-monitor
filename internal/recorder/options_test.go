package recorder

import (
	"bytes"
	"compress/gzip"
	"encoding/csv"
	"io"
	"testing"
)

// runOptionsSession starts a recorder over testColumns with the options Options
// assembles from spec, drives it for three ticks, and returns the main output
// bytes — the sidecar, when one is requested, lands in the writer the test's
// open closure returns.
func runOptionsSession(t *testing.T, spec OptionsSpec, snap ProcessSnapshot, open func() io.WriteCloser) []byte {
	t.Helper()
	rec := New(testColumns(), Options(spec, snap, open)...)
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

func TestOptionsDefaultSessionIsPlainCSV(t *testing.T) {
	out := runOptionsSession(t, OptionsSpec{}, nil, nil)
	if bytes.HasPrefix(out, gzipMagic) {
		t.Fatal("default spec session is gzip-compressed; expected plain CSV")
	}
}

func TestOptionsCompactWritesGzip(t *testing.T) {
	plain := runOptionsSession(t, OptionsSpec{}, nil, nil)
	compact := runOptionsSession(t, OptionsSpec{Compact: true}, nil, nil)

	if !bytes.HasPrefix(compact, gzipMagic) {
		t.Fatal("compact spec output is not a gzip stream")
	}
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

func TestOptionsTopNSidecarWritten(t *testing.T) {
	sidecar := &nopCloser{Buffer: &bytes.Buffer{}}
	main := runOptionsSession(t, OptionsSpec{TopN: 2}, testSnapshot, func() io.WriteCloser { return sidecar })
	if bytes.HasPrefix(main, gzipMagic) {
		t.Fatal("sidecar spec session unexpectedly gzip-compressed")
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
	// testSnapshot has two processes; three ticks → six snapshot rows.
	if want := 3 * 2; len(rows[1:]) != want {
		t.Fatalf("sidecar rows = %d, want %d", len(rows[1:]), want)
	}
	// The first snapshot row is the busiest process, sharing the metric timestamp.
	wantRow := []string{"2026-07-08T12:00:00Z", "42", "bash", "12.5", "2048"}
	if !equal(rows[1], wantRow) {
		t.Fatalf("first snapshot row = %v, want %v", rows[1], wantRow)
	}
}

func TestOptionsCompactWithSidecar(t *testing.T) {
	sidecar := &nopCloser{Buffer: &bytes.Buffer{}}
	main := runOptionsSession(t, OptionsSpec{Compact: true, TopN: 2}, testSnapshot, func() io.WriteCloser { return sidecar })
	if !bytes.HasPrefix(main, gzipMagic) {
		t.Fatal("compact spec output is not a gzip stream")
	}
	rows, err := csv.NewReader(bytes.NewReader(sidecar.Bytes())).ReadAll()
	if err != nil {
		t.Fatalf("sidecar is not valid CSV: %v", err)
	}
	if len(rows) != 7 { // header + six snapshot rows
		t.Fatalf("sidecar rows = %d, want 7", len(rows))
	}
}

func TestOptionsDropsSidecarWhenSnapshotNil(t *testing.T) {
	// A nil snapshot (process collector failed to start) must drop the sidecar
	// entirely, not record an empty file or panic on the nil sample.
	opened := false
	main := runOptionsSession(t, OptionsSpec{TopN: 5}, nil, func() io.WriteCloser {
		opened = true
		return &nopCloser{Buffer: &bytes.Buffer{}}
	})
	if opened {
		t.Fatal("sidecar opened despite a nil snapshot")
	}
	if bytes.HasPrefix(main, gzipMagic) {
		t.Fatal("unexpected gzip output")
	}
}

func TestParseTopN(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"   ", 0},
		{"0", 0},
		{"-4", 0},
		{"abc", 0},
		{"5", 5},
		{" 7 ", 7},
	}
	for _, c := range cases {
		if got := ParseTopN(c.in); got != c.want {
			t.Errorf("ParseTopN(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestTopSamples(t *testing.T) {
	in := []ProcessSample{
		{PID: 1, Name: "a", CPU: 3.5},
		{PID: 2, Name: "b", CPU: 99},
		{PID: 3, Name: "c", CPU: 40},
	}
	// Busiest first, capped at n.
	got := TopSamples(in, 2)
	if want := []int32{2, 3}; !pidsEqual(got, want) {
		t.Errorf("TopSamples top 2 = %v, want PIDs %v", pids(got), want)
	}
	// The input slice is not mutated.
	if in[0].PID != 1 || in[1].PID != 2 || in[2].PID != 3 {
		t.Errorf("TopSamples mutated its input: %v", pids(in))
	}
	// n larger than the sample passes everything through, sorted.
	all := TopSamples(in, 9)
	if want := []int32{2, 3, 1}; !pidsEqual(all, want) {
		t.Errorf("TopSamples uncapped = %v, want PIDs %v", pids(all), want)
	}
	// n <= 0 and empty samples mean no sidecar content.
	if got := TopSamples(in, 0); got != nil {
		t.Errorf("TopSamples n=0 = %v, want nil", got)
	}
	if got := TopSamples(nil, 2); got != nil {
		t.Errorf("TopSamples empty = %v, want nil", got)
	}
	if got := TopSamples(in, -1); got != nil {
		t.Errorf("TopSamples n=-1 = %v, want nil", got)
	}
}

// pids extracts the PID column for a human-readable comparison.
func pids(ps []ProcessSample) []int32 {
	out := make([]int32, len(ps))
	for i, p := range ps {
		out[i] = p.PID
	}
	return out
}

func pidsEqual(ps []ProcessSample, want []int32) bool {
	if len(ps) != len(want) {
		return false
	}
	for i := range ps {
		if ps[i].PID != want[i] {
			return false
		}
	}
	return true
}
