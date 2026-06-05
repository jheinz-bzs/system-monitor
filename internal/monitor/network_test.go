package monitor

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

// fakeNetSampler returns successive readings on each call, then repeats the
// last one. It records how many times it was invoked.
type fakeNetSampler struct {
	readings []netReading
	calls    int
}

func (f *fakeNetSampler) sample(ctx context.Context) (netReading, error) {
	r := f.readings[min(f.calls, len(f.readings)-1)]
	f.calls++
	return r, nil
}

func TestNewNetworkCollectorSeedsRates(t *testing.T) {
	f := &fakeNetSampler{readings: []netReading{{bytesSent: 100, bytesRecv: 200}}}

	c := NewNetworkCollector(context.Background(), withNetSampler(f.sample), withNetClock(steppingClock(clockStart, time.Second)))
	if c == nil {
		t.Fatal("NewNetworkCollector returned nil")
	}
	if got := c.UploadRate(); !reflect.DeepEqual(got, []uint64{0}) {
		t.Errorf("UploadRate() = %v, want [0] (seed has no prior delta)", got)
	}
	if got := c.DownloadRate(); !reflect.DeepEqual(got, []uint64{0}) {
		t.Errorf("DownloadRate() = %v, want [0] (seed has no prior delta)", got)
	}
	if got := c.TotalRate(); !reflect.DeepEqual(got, []uint64{0}) {
		t.Errorf("TotalRate() = %v, want [0] (seed has no prior delta)", got)
	}
}

func TestNewNetworkCollectorReturnsNilOnSamplerFailure(t *testing.T) {
	sample := func(ctx context.Context) (netReading, error) {
		return netReading{}, errors.New("boom")
	}
	if c := NewNetworkCollector(context.Background(), withNetSampler(sample), withNetClock(steppingClock(clockStart, time.Second))); c != nil {
		t.Fatal("NewNetworkCollector did not return nil on sampler error")
	}
}

func TestNetworkCollectComputesRatesAndTotalFromDelta(t *testing.T) {
	f := &fakeNetSampler{readings: []netReading{
		{bytesSent: 1000, bytesRecv: 500},
		{bytesSent: 3000, bytesRecv: 800},
		{bytesSent: 6000, bytesRecv: 800},
	}}

	c := NewNetworkCollector(context.Background(), withNetSampler(f.sample), withNetClock(steppingClock(clockStart, time.Second)))
	if c == nil {
		t.Fatal("NewNetworkCollector returned nil")
	}
	for i := 0; i < 2; i++ {
		if err := c.Collect(context.Background()); err != nil {
			t.Fatalf("Collect: %v", err)
		}
	}

	if got, want := c.UploadRate(), []uint64{0, 2000, 3000}; !reflect.DeepEqual(got, want) {
		t.Errorf("UploadRate() = %v, want %v", got, want)
	}
	if got, want := c.DownloadRate(), []uint64{0, 300, 0}; !reflect.DeepEqual(got, want) {
		t.Errorf("DownloadRate() = %v, want %v", got, want)
	}
	// total = upload + download at each step.
	if got, want := c.TotalRate(), []uint64{0, 2300, 3000}; !reflect.DeepEqual(got, want) {
		t.Errorf("TotalRate() = %v, want %v", got, want)
	}
}

func TestNetworkCollectDividesByElapsedTime(t *testing.T) {
	f := &fakeNetSampler{readings: []netReading{
		{bytesSent: 0, bytesRecv: 0},
		{bytesSent: 2000, bytesRecv: 4000},
	}}

	// A 2-second step means the 2000/4000-byte delta becomes a 1000/2000 B/s rate.
	c := NewNetworkCollector(context.Background(), withNetSampler(f.sample), withNetClock(steppingClock(clockStart, 2*time.Second)))
	if c == nil {
		t.Fatal("NewNetworkCollector returned nil")
	}
	if err := c.Collect(context.Background()); err != nil {
		t.Fatalf("Collect: %v", err)
	}

	if got, want := c.UploadRate(), []uint64{0, 1000}; !reflect.DeepEqual(got, want) {
		t.Errorf("UploadRate() = %v, want %v", got, want)
	}
	if got, want := c.DownloadRate(), []uint64{0, 2000}; !reflect.DeepEqual(got, want) {
		t.Errorf("DownloadRate() = %v, want %v", got, want)
	}
	if got, want := c.TotalRate(), []uint64{0, 3000}; !reflect.DeepEqual(got, want) {
		t.Errorf("TotalRate() = %v, want %v", got, want)
	}
}

func TestNetworkCollectCounterWrapYieldsZeroRate(t *testing.T) {
	f := &fakeNetSampler{readings: []netReading{
		{bytesSent: 5000, bytesRecv: 5000},
		{bytesSent: 1000, bytesRecv: 1000}, // counters reset/wrapped
	}}

	c := NewNetworkCollector(context.Background(), withNetSampler(f.sample), withNetClock(steppingClock(clockStart, time.Second)))
	if c == nil {
		t.Fatal("NewNetworkCollector returned nil")
	}
	if err := c.Collect(context.Background()); err != nil {
		t.Fatalf("Collect: %v", err)
	}

	if got, want := c.UploadRate(), []uint64{0, 0}; !reflect.DeepEqual(got, want) {
		t.Errorf("UploadRate() = %v, want %v", got, want)
	}
	if got, want := c.DownloadRate(), []uint64{0, 0}; !reflect.DeepEqual(got, want) {
		t.Errorf("DownloadRate() = %v, want %v", got, want)
	}
	if got, want := c.TotalRate(), []uint64{0, 0}; !reflect.DeepEqual(got, want) {
		t.Errorf("TotalRate() = %v, want %v", got, want)
	}
}

func TestNetworkCollectReturnsErrorOnSamplerFailure(t *testing.T) {
	calls := 0
	sample := func(ctx context.Context) (netReading, error) {
		calls++
		if calls == 1 {
			return netReading{bytesSent: 1, bytesRecv: 1}, nil // seed succeeds
		}
		return netReading{}, errors.New("boom")
	}

	c := NewNetworkCollector(context.Background(), withNetSampler(sample), withNetClock(steppingClock(clockStart, time.Second)))
	if c == nil {
		t.Fatal("NewNetworkCollector returned nil")
	}
	if err := c.Collect(context.Background()); err == nil {
		t.Fatal("Collect did not return an error when the sampler failed")
	}
}

func TestNetworkReadMethodsReturnIndependentCopies(t *testing.T) {
	f := &fakeNetSampler{readings: []netReading{{bytesSent: 10, bytesRecv: 20}}}

	c := NewNetworkCollector(context.Background(), withNetSampler(f.sample), withNetClock(steppingClock(clockStart, time.Second)))
	if c == nil {
		t.Fatal("NewNetworkCollector returned nil")
	}

	c.UploadRate()[0] = 999
	c.DownloadRate()[0] = 999
	c.TotalRate()[0] = 999

	if got := c.UploadRate(); got[0] == 999 {
		t.Errorf("UploadRate() exposed mutable internal state: %v", got)
	}
	if got := c.DownloadRate(); got[0] == 999 {
		t.Errorf("DownloadRate() exposed mutable internal state: %v", got)
	}
	if got := c.TotalRate(); got[0] == 999 {
		t.Errorf("TotalRate() exposed mutable internal state: %v", got)
	}
}
