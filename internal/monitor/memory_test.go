package monitor

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

// fakeMemSampler returns successive readings on each call, then repeats the
// last one. It records how many times it was invoked.
type fakeMemSampler struct {
	readings []memReading
	calls    int
}

func (f *fakeMemSampler) sample(ctx context.Context) (memReading, error) {
	r := f.readings[min(f.calls, len(f.readings)-1)]
	f.calls++
	return r, nil
}

func TestNewMemoryCollectorRecordsTotalAndSeedsBuffers(t *testing.T) {
	f := &fakeMemSampler{readings: []memReading{{total: 16000, used: 4000, cached: 2000, free: 10000}}}

	c, err := newMemoryCollector(context.Background(), f.sample)
	if err != nil {
		t.Fatalf("newMemoryCollector: %v", err)
	}
	if got := c.Total(); got != 16000 {
		t.Errorf("Total() = %d, want 16000", got)
	}
	if got := c.Used(); !reflect.DeepEqual(got, []uint64{4000}) {
		t.Errorf("Used() = %v, want [4000]", got)
	}
	if got := c.Cached(); !reflect.DeepEqual(got, []uint64{2000}) {
		t.Errorf("Cached() = %v, want [2000]", got)
	}
	if got := c.Free(); !reflect.DeepEqual(got, []uint64{10000}) {
		t.Errorf("Free() = %v, want [10000]", got)
	}
}

func TestNewMemoryCollectorErrorsOnSamplerFailure(t *testing.T) {
	sample := func(ctx context.Context) (memReading, error) {
		return memReading{}, errors.New("boom")
	}
	if _, err := newMemoryCollector(context.Background(), sample); err == nil {
		t.Fatal("newMemoryCollector did not return an error")
	}
}

func TestCollectAppendsToEachBuffer(t *testing.T) {
	f := &fakeMemSampler{readings: []memReading{
		{total: 16000, used: 1000, cached: 100, free: 14900},
		{total: 16000, used: 2000, cached: 200, free: 13800},
		{total: 16000, used: 3000, cached: 300, free: 12700},
	}}

	c, err := newMemoryCollector(context.Background(), f.sample)
	if err != nil {
		t.Fatalf("newMemoryCollector: %v", err)
	}
	for i := 0; i < 2; i++ {
		if err := c.Collect(context.Background()); err != nil {
			t.Fatalf("Collect: %v", err)
		}
	}

	if got, want := c.Used(), []uint64{1000, 2000, 3000}; !reflect.DeepEqual(got, want) {
		t.Errorf("Used() = %v, want %v", got, want)
	}
	if got, want := c.Cached(), []uint64{100, 200, 300}; !reflect.DeepEqual(got, want) {
		t.Errorf("Cached() = %v, want %v", got, want)
	}
	if got, want := c.Free(), []uint64{14900, 13800, 12700}; !reflect.DeepEqual(got, want) {
		t.Errorf("Free() = %v, want %v", got, want)
	}
}

func TestMemoryCollectReturnsErrorOnSamplerFailure(t *testing.T) {
	calls := 0
	sample := func(ctx context.Context) (memReading, error) {
		calls++
		if calls == 1 {
			return memReading{total: 16000, used: 1000, cached: 100, free: 14900}, nil // seed succeeds
		}
		return memReading{}, errors.New("boom")
	}

	c, err := newMemoryCollector(context.Background(), sample)
	if err != nil {
		t.Fatalf("newMemoryCollector: %v", err)
	}
	if err := c.Collect(context.Background()); err == nil {
		t.Fatal("Collect did not return an error when the sampler failed")
	}
}

func TestMemoryReadMethodsReturnIndependentCopies(t *testing.T) {
	f := &fakeMemSampler{readings: []memReading{{total: 16000, used: 4000, cached: 2000, free: 10000}}}

	c, err := newMemoryCollector(context.Background(), f.sample)
	if err != nil {
		t.Fatalf("newMemoryCollector: %v", err)
	}

	c.Used()[0] = 999
	c.Cached()[0] = 999
	c.Free()[0] = 999

	if got := c.Used(); got[0] == 999 {
		t.Errorf("Used() exposed mutable internal state: %v", got)
	}
	if got := c.Cached(); got[0] == 999 {
		t.Errorf("Cached() exposed mutable internal state: %v", got)
	}
	if got := c.Free(); got[0] == 999 {
		t.Errorf("Free() exposed mutable internal state: %v", got)
	}
}
