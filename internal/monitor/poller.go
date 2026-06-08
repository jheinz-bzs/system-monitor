package monitor

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Collector is the seam the Poller drives. Each registered collector samples
// fresh data into its own ring buffers or snapshots when Collect is called.
// Every collector in this package satisfies it, and new metrics can be added
// without touching the Poller.
type Collector interface {
	Collect(ctx context.Context) error
}

// Poller is the application heartbeat: on every tick it calls Collect on each
// registered collector so ring buffers and snapshots stay continuously fresh.
// It owns a single goroutine, started by Start and torn down by Stop, giving
// the app clean lifecycle control with no goroutine leaks.
type Poller struct {
	interval   time.Duration
	collectors []Collector
	onTick     func()

	mu     sync.Mutex
	cancel context.CancelFunc
	done   chan struct{}
}

// NewPoller builds a Poller that ticks every interval and drives the given
// collectors. The interval is supplied here rather than hardcoded so callers
// choose the polling frequency.
func NewPoller(interval time.Duration, collectors ...Collector) *Poller {
	return &Poller{
		interval:   interval,
		collectors: collectors,
	}
}

// OnTick registers a callback invoked once after every collection pass —
// including the immediate one on Start — so the data and any reaction to it
// (e.g. a UI redraw) share a single clock. Without this, a separate ticker on
// another goroutine drifts against the poll ticker and the two beat against
// each other, producing uneven update cadence. Call before Start; the callback
// runs on the poller goroutine, so any UI work inside it must be marshalled
// onto the UI thread by the callback itself.
func (p *Poller) OnTick(fn func()) {
	p.onTick = fn
}

// Start launches the polling goroutine. It collects once immediately so the
// buffers populate without waiting a full interval, then again on every tick.
// The goroutine exits when ctx is cancelled or Stop is called. Calling Start on
// an already-running Poller is a no-op.
func (p *Poller) Start(ctx context.Context) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cancel != nil {
		return
	}

	ctx, cancel := context.WithCancel(ctx)
	p.cancel = cancel
	p.done = make(chan struct{})
	go p.run(ctx, p.done)
}

// Stop halts polling and blocks until the goroutine has fully exited, so after
// it returns no collection is in flight and no goroutine is leaked. Calling
// Stop when not running is a no-op.
func (p *Poller) Stop() {
	p.mu.Lock()
	cancel, done := p.cancel, p.done
	p.cancel, p.done = nil, nil
	p.mu.Unlock()

	if cancel == nil {
		return
	}
	cancel()
	<-done
}

// run drives collection until ctx is cancelled, closing done on exit so Stop
// can wait for a clean shutdown.
func (p *Poller) run(ctx context.Context, done chan struct{}) {
	defer close(done)

	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	p.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p.tick(ctx)
		}
	}
}

// tick performs one collection pass and then fires the registered callback, so
// observers react exactly once per pass, right after fresh data has landed.
func (p *Poller) tick(ctx context.Context) {
	p.collectAll(ctx)
	if p.onTick != nil {
		p.onTick()
	}
}

// collectAll calls Collect on every collector in turn. A failure is logged and
// skipped so one collector cannot stall the ticker or starve the others.
func (p *Poller) collectAll(ctx context.Context) {
	for _, c := range p.collectors {
		if err := c.Collect(ctx); err != nil {
			slog.Error("collector failed", "err", err)
		}
	}
}
