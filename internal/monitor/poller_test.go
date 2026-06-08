package monitor

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"testing"
	"time"
)

// countingCollector records how many times Collect was called and optionally
// returns a fixed error each time. It is safe for concurrent use because the
// poller goroutine writes while the test reads.
type countingCollector struct {
	mu    sync.Mutex
	calls int
	err   error
}

func (c *countingCollector) Collect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls++
	return c.err
}

func (c *countingCollector) count() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.calls
}

// waitFor polls cond until it holds or the timeout elapses, failing the test on
// timeout. It keeps the ticker-driven tests deterministic without sleeping for
// a fixed, flaky duration.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("condition not met within timeout")
}

func TestPollerCollectsImmediatelyOnStart(t *testing.T) {
	c := &countingCollector{}
	// A long interval guarantees only the immediate collect can fire.
	p := NewPoller(time.Hour, c)
	p.Start(context.Background())
	defer p.Stop()

	waitFor(t, func() bool { return c.count() >= 1 })
	if got := c.count(); got != 1 {
		t.Errorf("count = %d, want exactly 1 immediate collect", got)
	}
}

func TestPollerTicksEveryCollector(t *testing.T) {
	a, b := &countingCollector{}, &countingCollector{}
	p := NewPoller(5*time.Millisecond, a, b)
	p.Start(context.Background())
	defer p.Stop()

	waitFor(t, func() bool { return a.count() >= 3 && b.count() >= 3 })
}

func TestPollerLogsCollectorErrorAndContinues(t *testing.T) {
	bad := &countingCollector{err: errors.New("boom")}
	good := &countingCollector{}
	p := NewPoller(5*time.Millisecond, bad, good)
	p.Start(context.Background())
	defer p.Stop()

	// The healthy collector keeps ticking despite its neighbour erroring...
	waitFor(t, func() bool { return good.count() >= 3 })
	// ...and the failing collector keeps being polled rather than being dropped.
	if bad.count() < 3 {
		t.Errorf("erroring collector stopped being polled: got %d calls", bad.count())
	}
}

func TestPollerStopIsIdempotentAndSafeWithoutStart(t *testing.T) {
	p := NewPoller(5*time.Millisecond, &countingCollector{})
	p.Stop() // never started

	p.Start(context.Background())
	p.Stop()
	p.Stop() // second stop is a no-op
}

func TestPollerStopLeavesNoGoroutineLeak(t *testing.T) {
	baseline := runtime.NumGoroutine()

	p := NewPoller(5*time.Millisecond, &countingCollector{})
	p.Start(context.Background())
	if got := runtime.NumGoroutine(); got <= baseline {
		t.Fatalf("expected a running goroutine after Start: baseline %d, got %d", baseline, got)
	}

	p.Stop()
	waitFor(t, func() bool { return runtime.NumGoroutine() <= baseline })
}

func TestPollerStopsOnContextCancel(t *testing.T) {
	baseline := runtime.NumGoroutine()
	ctx, cancel := context.WithCancel(context.Background())

	c := &countingCollector{}
	p := NewPoller(5*time.Millisecond, c)
	p.Start(ctx)
	waitFor(t, func() bool { return c.count() >= 1 })

	cancel()
	waitFor(t, func() bool { return runtime.NumGoroutine() <= baseline })
}
