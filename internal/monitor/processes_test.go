package monitor

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

// fakeProcSampler returns successive process snapshots on each call, then
// repeats the last one.
type fakeProcSampler struct {
	readings [][]ProcessInfo
	calls    int
}

func (f *fakeProcSampler) sample(ctx context.Context) ([]ProcessInfo, error) {
	r := f.readings[min(f.calls, len(f.readings)-1)]
	f.calls++
	return r, nil
}

// fakeConnSampler returns successive connection snapshots on each call, then
// repeats the last one.
type fakeConnSampler struct {
	readings [][]ConnectionInfo
	calls    int
}

func (f *fakeConnSampler) sample(ctx context.Context) ([]ConnectionInfo, error) {
	r := f.readings[min(f.calls, len(f.readings)-1)]
	f.calls++
	return r, nil
}

func okConns() connSampler {
	f := &fakeConnSampler{readings: [][]ConnectionInfo{nil}}
	return f.sample
}

func okProcs() processSampler {
	f := &fakeProcSampler{readings: [][]ProcessInfo{nil}}
	return f.sample
}

func TestNewProcessCollectorSeedsSnapshots(t *testing.T) {
	procs := &fakeProcSampler{readings: [][]ProcessInfo{{{PID: 1, Name: "init"}}}}
	conns := &fakeConnSampler{readings: [][]ConnectionInfo{{
		{Protocol: "tcp", LocalAddr: "0.0.0.0:80", State: "LISTEN", PID: 1},
	}}}

	c, err := NewProcessCollector(context.Background(), withProcessSampler(procs.sample), withConnSampler(conns.sample))
	if err != nil {
		t.Fatalf("NewProcessCollector: %v", err)
	}

	if got := c.Processes(); len(got) != 1 || got[0].PID != 1 {
		t.Errorf("Processes() = %v, want one process with PID 1", got)
	}
	if got := c.Connections(); len(got) != 1 {
		t.Errorf("Connections() = %v, want one connection", got)
	}
	if got := c.Ports(); len(got) != 1 || got[0].Port != 80 {
		t.Errorf("Ports() = %v, want one port 80 derived from the LISTEN socket", got)
	}
}

func TestNewProcessCollectorErrorsOnProcessSamplerFailure(t *testing.T) {
	boom := func(ctx context.Context) ([]ProcessInfo, error) { return nil, errors.New("boom") }
	if _, err := NewProcessCollector(context.Background(), withProcessSampler(boom), withConnSampler(okConns())); err == nil {
		t.Fatal("NewProcessCollector did not return an error on process sampler failure")
	}
}

func TestNewProcessCollectorErrorsOnConnSamplerFailure(t *testing.T) {
	boom := func(ctx context.Context) ([]ConnectionInfo, error) { return nil, errors.New("boom") }
	if _, err := NewProcessCollector(context.Background(), withProcessSampler(okProcs()), withConnSampler(boom)); err == nil {
		t.Fatal("NewProcessCollector did not return an error on connection sampler failure")
	}
}

func TestCollectReplacesSnapshots(t *testing.T) {
	procs := &fakeProcSampler{readings: [][]ProcessInfo{
		{{PID: 1}},
		{{PID: 2}, {PID: 3}},
	}}
	conns := &fakeConnSampler{readings: [][]ConnectionInfo{
		{{Protocol: "tcp", LocalAddr: "0.0.0.0:22", State: "LISTEN", PID: 1}},
		nil,
	}}

	c, err := NewProcessCollector(context.Background(), withProcessSampler(procs.sample), withConnSampler(conns.sample))
	if err != nil {
		t.Fatalf("NewProcessCollector: %v", err)
	}
	if err := c.Collect(context.Background()); err != nil {
		t.Fatalf("Collect: %v", err)
	}

	if got := c.Processes(); len(got) != 2 {
		t.Errorf("Processes() = %v, want the second snapshot of 2 processes", got)
	}
	if got := c.Ports(); len(got) != 0 {
		t.Errorf("Ports() = %v, want empty after the second snapshot had no listeners", got)
	}
}

func TestProcessCollectReturnsErrorOnSamplerFailure(t *testing.T) {
	calls := 0
	procs := func(ctx context.Context) ([]ProcessInfo, error) {
		calls++
		if calls == 1 {
			return nil, nil // seed succeeds
		}
		return nil, errors.New("boom")
	}

	c, err := NewProcessCollector(context.Background(), withProcessSampler(procs), withConnSampler(okConns()))
	if err != nil {
		t.Fatalf("NewProcessCollector: %v", err)
	}
	if err := c.Collect(context.Background()); err == nil {
		t.Fatal("Collect did not return an error when the sampler failed")
	}
}

func TestProcessSnapshotRetainsZeroValueFields(t *testing.T) {
	// A permission-restricted process surfaces with empty/zero fields but a valid
	// PID; it must still be present in the snapshot.
	procs := &fakeProcSampler{readings: [][]ProcessInfo{{{PID: 42}}}}

	c, err := NewProcessCollector(context.Background(), withProcessSampler(procs.sample), withConnSampler(okConns()))
	if err != nil {
		t.Fatalf("NewProcessCollector: %v", err)
	}

	got := c.Processes()
	if len(got) != 1 {
		t.Fatalf("Processes() = %v, want the restricted process retained", got)
	}
	want := ProcessInfo{PID: 42}
	if got[0] != want {
		t.Errorf("Processes()[0] = %+v, want %+v", got[0], want)
	}
}

func TestDeriveListeningPorts(t *testing.T) {
	conns := []ConnectionInfo{
		{Protocol: "tcp", LocalAddr: "0.0.0.0:8080", State: "LISTEN", PID: 10},
		{Protocol: "tcp", LocalAddr: "10.0.0.1:55000", RemoteAddr: "1.2.3.4:443", State: "ESTABLISHED", PID: 11},
		{Protocol: "udp", LocalAddr: "0.0.0.0:53", RemoteAddr: "", PID: 12},
		{Protocol: "udp", LocalAddr: "10.0.0.1:51000", RemoteAddr: "1.2.3.4:53", PID: 13},
	}

	got := deriveListeningPorts(conns)
	want := []PortInfo{
		{Port: 8080, Protocol: "tcp", LocalAddr: "0.0.0.0:8080", PID: 10},
		{Port: 53, Protocol: "udp", LocalAddr: "0.0.0.0:53", PID: 12},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("deriveListeningPorts() = %v, want %v", got, want)
	}
}

func TestProcessReadMethodsReturnIndependentCopies(t *testing.T) {
	procs := &fakeProcSampler{readings: [][]ProcessInfo{{{PID: 1}}}}
	conns := &fakeConnSampler{readings: [][]ConnectionInfo{{
		{Protocol: "tcp", LocalAddr: "0.0.0.0:80", State: "LISTEN", PID: 1},
	}}}

	c, err := NewProcessCollector(context.Background(), withProcessSampler(procs.sample), withConnSampler(conns.sample))
	if err != nil {
		t.Fatalf("NewProcessCollector: %v", err)
	}

	c.Processes()[0].PID = 999
	c.Ports()[0].PID = 999
	c.Connections()[0].PID = 999

	if got := c.Processes(); got[0].PID == 999 {
		t.Errorf("Processes() exposed mutable internal state: %v", got)
	}
	if got := c.Ports(); got[0].PID == 999 {
		t.Errorf("Ports() exposed mutable internal state: %v", got)
	}
	if got := c.Connections(); got[0].PID == 999 {
		t.Errorf("Connections() exposed mutable internal state: %v", got)
	}
}
