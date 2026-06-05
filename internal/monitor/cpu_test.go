package monitor

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

// fakeSampler returns successive readings on each call, then repeats the last
// one. It records how many times it was invoked.
type fakeSampler struct {
	readings [][]float64
	calls    int
}

func (f *fakeSampler) sample(ctx context.Context) ([]float64, error) {
	r := f.readings[min(f.calls, len(f.readings)-1)]
	f.calls++
	return r, nil
}

func TestNewCPUCollectorSizesBuffersToCoreCount(t *testing.T) {
	f := &fakeSampler{readings: [][]float64{{10, 20, 30, 40}}}

	c := NewCPUCollector(context.Background(), withCPUSampler(f.sample))
	if c == nil {
		t.Fatal("NewCPUCollector returned nil")
	}
	if got := c.CoreCount(); got != 4 {
		t.Errorf("CoreCount() = %d, want 4", got)
	}
	if got := len(c.PerCore()); got != 4 {
		t.Errorf("len(PerCore()) = %d, want 4", got)
	}
}

func TestNewCPUCollectorSeedsBuffers(t *testing.T) {
	f := &fakeSampler{readings: [][]float64{{20, 40}}}

	c := NewCPUCollector(context.Background(), withCPUSampler(f.sample))
	if c == nil {
		t.Fatal("NewCPUCollector returned nil")
	}
	if got := c.Overall(); !reflect.DeepEqual(got, []float64{30}) {
		t.Errorf("Overall() = %v, want [30] (mean of seed)", got)
	}
	if got := c.PerCore(); !reflect.DeepEqual(got, [][]float64{{20}, {40}}) {
		t.Errorf("PerCore() = %v, want [[20] [40]]", got)
	}
}

func TestNewCPUCollectorReturnsNil(t *testing.T) {
	t.Run("sampler error", func(t *testing.T) {
		sample := func(ctx context.Context) ([]float64, error) {
			return nil, errors.New("boom")
		}
		if c := NewCPUCollector(context.Background(), withCPUSampler(sample)); c != nil {
			t.Fatal("NewCPUCollector did not return nil on sampler error")
		}
	})
	t.Run("no cores", func(t *testing.T) {
		sample := func(ctx context.Context) ([]float64, error) {
			return []float64{}, nil
		}
		if c := NewCPUCollector(context.Background(), withCPUSampler(sample)); c != nil {
			t.Fatal("NewCPUCollector did not return nil for zero cores")
		}
	})
}

func TestCollectDistributesPerCoreAndMean(t *testing.T) {
	f := &fakeSampler{readings: [][]float64{
		{0, 0},
		{10, 30},
		{50, 50},
	}}

	c := NewCPUCollector(context.Background(), withCPUSampler(f.sample))
	if c == nil {
		t.Fatal("NewCPUCollector returned nil")
	}
	for i := 0; i < 2; i++ {
		if err := c.Collect(context.Background()); err != nil {
			t.Fatalf("Collect: %v", err)
		}
	}

	wantPerCore := [][]float64{{0, 10, 50}, {0, 30, 50}}
	if got := c.PerCore(); !reflect.DeepEqual(got, wantPerCore) {
		t.Errorf("PerCore() = %v, want %v", got, wantPerCore)
	}
	wantOverall := []float64{0, 20, 50} // means of each reading
	if got := c.Overall(); !reflect.DeepEqual(got, wantOverall) {
		t.Errorf("Overall() = %v, want %v", got, wantOverall)
	}
}

func TestCollectReturnsErrorOnSamplerFailure(t *testing.T) {
	calls := 0
	sample := func(ctx context.Context) ([]float64, error) {
		calls++
		if calls == 1 {
			return []float64{1, 2}, nil // seed succeeds
		}
		return nil, errors.New("boom")
	}

	c := NewCPUCollector(context.Background(), withCPUSampler(sample))
	if c == nil {
		t.Fatal("NewCPUCollector returned nil")
	}
	if err := c.Collect(context.Background()); err == nil {
		t.Fatal("Collect did not return an error when the sampler failed")
	}
}

func TestCollectReturnsErrorOnCoreCountChange(t *testing.T) {
	f := &fakeSampler{readings: [][]float64{
		{1, 2, 3},
		{4, 5}, // a core disappeared
	}}

	c := NewCPUCollector(context.Background(), withCPUSampler(f.sample))
	if c == nil {
		t.Fatal("NewCPUCollector returned nil")
	}
	if err := c.Collect(context.Background()); err == nil {
		t.Fatal("Collect did not return an error on a core-count change")
	}
}

func TestReadMethodsReturnIndependentCopies(t *testing.T) {
	f := &fakeSampler{readings: [][]float64{{10, 20}}}

	c := NewCPUCollector(context.Background(), withCPUSampler(f.sample))
	if c == nil {
		t.Fatal("NewCPUCollector returned nil")
	}

	overall := c.Overall()
	if len(overall) > 0 {
		overall[0] = 999
	}
	perCore := c.PerCore()
	perCore[0][0] = 999

	if got := c.Overall(); len(got) == 0 || got[0] == 999 {
		t.Errorf("Overall() exposed mutable internal state: %v", got)
	}
	if got := c.PerCore(); got[0][0] == 999 {
		t.Errorf("PerCore() exposed mutable internal state: %v", got)
	}
}
