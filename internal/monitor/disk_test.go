package monitor

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

// fakeDiskSampler returns successive readings on each call, then repeats the
// last one. It records how many times it was invoked.
type fakeDiskSampler struct {
	readings []diskReading
	calls    int
}

func (f *fakeDiskSampler) sample(ctx context.Context) (diskReading, error) {
	r := f.readings[min(f.calls, len(f.readings)-1)]
	f.calls++
	return r, nil
}

// steppingClock returns a clock that yields start on its first call and advances
// by step on every subsequent call. With the collector calling now() once at
// construction and once per Collect, each Collect sees exactly step of elapsed
// time.
func steppingClock(start time.Time, step time.Duration) func() time.Time {
	cur := start
	first := true
	return func() time.Time {
		if first {
			first = false
			return cur
		}
		cur = cur.Add(step)
		return cur
	}
}

var clockStart = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

func TestNewDiskCollectorSeedsSnapshotAndRates(t *testing.T) {
	parts := []PartitionUsage{{Mountpoint: "/", Fstype: "ext4", Total: 1000, Used: 400}}
	f := &fakeDiskSampler{readings: []diskReading{{partitions: parts, readBytes: 100, writeBytes: 200}}}

	c := NewDiskCollector(context.Background(), withDiskSampler(f.sample), withDiskClock(steppingClock(clockStart, time.Second)))
	if c == nil {
		t.Fatal("NewDiskCollector returned nil")
	}
	if got := c.Usage(); !reflect.DeepEqual(got, parts) {
		t.Errorf("Usage() = %v, want %v", got, parts)
	}
	if got := c.ReadRate(); !reflect.DeepEqual(got, []uint64{0}) {
		t.Errorf("ReadRate() = %v, want [0] (seed has no prior delta)", got)
	}
	if got := c.WriteRate(); !reflect.DeepEqual(got, []uint64{0}) {
		t.Errorf("WriteRate() = %v, want [0] (seed has no prior delta)", got)
	}
}

func TestNewDiskCollectorReturnsNilOnSamplerFailure(t *testing.T) {
	sample := func(ctx context.Context) (diskReading, error) {
		return diskReading{}, errors.New("boom")
	}
	if c := NewDiskCollector(context.Background(), withDiskSampler(sample), withDiskClock(steppingClock(clockStart, time.Second))); c != nil {
		t.Fatal("NewDiskCollector did not return nil on sampler error")
	}
}

func TestCollectComputesRatesFromDelta(t *testing.T) {
	f := &fakeDiskSampler{readings: []diskReading{
		{readBytes: 1000, writeBytes: 500},
		{readBytes: 3000, writeBytes: 800},
		{readBytes: 6000, writeBytes: 800},
	}}

	c := NewDiskCollector(context.Background(), withDiskSampler(f.sample), withDiskClock(steppingClock(clockStart, time.Second)))
	if c == nil {
		t.Fatal("NewDiskCollector returned nil")
	}
	for i := 0; i < 2; i++ {
		if err := c.Collect(context.Background()); err != nil {
			t.Fatalf("Collect: %v", err)
		}
	}

	if got, want := c.ReadRate(), []uint64{0, 2000, 3000}; !reflect.DeepEqual(got, want) {
		t.Errorf("ReadRate() = %v, want %v", got, want)
	}
	if got, want := c.WriteRate(), []uint64{0, 300, 0}; !reflect.DeepEqual(got, want) {
		t.Errorf("WriteRate() = %v, want %v", got, want)
	}
}

func TestCollectDividesByElapsedTime(t *testing.T) {
	f := &fakeDiskSampler{readings: []diskReading{
		{readBytes: 0, writeBytes: 0},
		{readBytes: 2000, writeBytes: 4000},
	}}

	// A 2-second step means the 2000/4000-byte delta becomes a 1000/2000 B/s rate.
	c := NewDiskCollector(context.Background(), withDiskSampler(f.sample), withDiskClock(steppingClock(clockStart, 2*time.Second)))
	if c == nil {
		t.Fatal("NewDiskCollector returned nil")
	}
	if err := c.Collect(context.Background()); err != nil {
		t.Fatalf("Collect: %v", err)
	}

	if got, want := c.ReadRate(), []uint64{0, 1000}; !reflect.DeepEqual(got, want) {
		t.Errorf("ReadRate() = %v, want %v", got, want)
	}
	if got, want := c.WriteRate(), []uint64{0, 2000}; !reflect.DeepEqual(got, want) {
		t.Errorf("WriteRate() = %v, want %v", got, want)
	}
}

func TestCollectCounterWrapYieldsZeroRate(t *testing.T) {
	f := &fakeDiskSampler{readings: []diskReading{
		{readBytes: 5000, writeBytes: 5000},
		{readBytes: 1000, writeBytes: 1000}, // counters reset/wrapped
	}}

	c := NewDiskCollector(context.Background(), withDiskSampler(f.sample), withDiskClock(steppingClock(clockStart, time.Second)))
	if c == nil {
		t.Fatal("NewDiskCollector returned nil")
	}
	if err := c.Collect(context.Background()); err != nil {
		t.Fatalf("Collect: %v", err)
	}

	if got, want := c.ReadRate(), []uint64{0, 0}; !reflect.DeepEqual(got, want) {
		t.Errorf("ReadRate() = %v, want %v", got, want)
	}
	if got, want := c.WriteRate(), []uint64{0, 0}; !reflect.DeepEqual(got, want) {
		t.Errorf("WriteRate() = %v, want %v", got, want)
	}
}

func TestCollectUpdatesUsageSnapshot(t *testing.T) {
	first := []PartitionUsage{{Mountpoint: "/", Total: 1000, Used: 400}}
	second := []PartitionUsage{
		{Mountpoint: "/", Total: 1000, Used: 600},
		{Mountpoint: "/data", Total: 2000, Used: 100},
	}
	f := &fakeDiskSampler{readings: []diskReading{
		{partitions: first},
		{partitions: second},
	}}

	c := NewDiskCollector(context.Background(), withDiskSampler(f.sample), withDiskClock(steppingClock(clockStart, time.Second)))
	if c == nil {
		t.Fatal("NewDiskCollector returned nil")
	}
	if err := c.Collect(context.Background()); err != nil {
		t.Fatalf("Collect: %v", err)
	}

	if got := c.Usage(); !reflect.DeepEqual(got, second) {
		t.Errorf("Usage() = %v, want %v", got, second)
	}
}

func TestDiskCollectReturnsErrorOnSamplerFailure(t *testing.T) {
	calls := 0
	sample := func(ctx context.Context) (diskReading, error) {
		calls++
		if calls == 1 {
			return diskReading{readBytes: 1, writeBytes: 1}, nil // seed succeeds
		}
		return diskReading{}, errors.New("boom")
	}

	c := NewDiskCollector(context.Background(), withDiskSampler(sample), withDiskClock(steppingClock(clockStart, time.Second)))
	if c == nil {
		t.Fatal("NewDiskCollector returned nil")
	}
	if err := c.Collect(context.Background()); err == nil {
		t.Fatal("Collect did not return an error when the sampler failed")
	}
}

func TestDiskReadMethodsReturnIndependentCopies(t *testing.T) {
	parts := []PartitionUsage{{Mountpoint: "/", Total: 1000, Used: 400}}
	f := &fakeDiskSampler{readings: []diskReading{{partitions: parts, readBytes: 10, writeBytes: 20}}}

	c := NewDiskCollector(context.Background(), withDiskSampler(f.sample), withDiskClock(steppingClock(clockStart, time.Second)))
	if c == nil {
		t.Fatal("NewDiskCollector returned nil")
	}

	c.Usage()[0].Used = 999
	c.ReadRate()[0] = 999
	c.WriteRate()[0] = 999

	if got := c.Usage(); got[0].Used == 999 {
		t.Errorf("Usage() exposed mutable internal state: %v", got)
	}
	if got := c.ReadRate(); got[0] == 999 {
		t.Errorf("ReadRate() exposed mutable internal state: %v", got)
	}
	if got := c.WriteRate(); got[0] == 999 {
		t.Errorf("WriteRate() exposed mutable internal state: %v", got)
	}
}
