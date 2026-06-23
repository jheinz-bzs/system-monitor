package monitor

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/shirou/gopsutil/v4/mem"

	"github.com/josephheinz/system-monitor/internal/metrics"
	"github.com/josephheinz/system-monitor/internal/ringbuffer"
)

// memOption configures a MemoryCollector at construction. It exists so tests can
// inject a sampler without a separate constructor; production code uses the
// defaults.
type memOption func(*MemoryCollector)

// withMemSampler overrides the memory sampler. Tests use it to supply readings
// without real hardware.
func withMemSampler(s memSampler) memOption {
	return func(c *MemoryCollector) { c.sample = s }
}

// memReading is one snapshot of memory in bytes. It is the value the collector
// samples through so tests can supply readings without real hardware.
type memReading struct {
	total     uint64
	used      uint64
	cached    uint64
	free      uint64
	swapTotal uint64
	swapUsed  uint64
}

// memSampler returns a single memory reading. It is the seam the collector
// samples through.
type memSampler func(ctx context.Context) (memReading, error)

// defaultMemSampler reads virtual memory stats via gopsutil. Swap is best-effort:
// a swap read failure leaves the swap fields zero rather than failing the whole
// reading, so a machine with no/locked swap still reports physical memory.
func defaultMemSampler(ctx context.Context) (memReading, error) {
	stat, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return memReading{}, fmt.Errorf("sampling memory: %w", err)
	}
	r := memReading{
		total:  stat.Total,
		used:   stat.Used,
		cached: stat.Cached,
		free:   stat.Free,
	}
	if swap, err := mem.SwapMemoryWithContext(ctx); err == nil {
		r.swapTotal = swap.Total
		r.swapUsed = swap.Used
	} else {
		slog.Warn("sampling swap", "err", err)
	}
	return r, nil
}

// MemoryCollector samples memory usage on each Collect and stores the history
// in ring buffers: one each for used, cached, and free bytes. Total physical
// memory is a static field because it does not change. Reads and writes are
// safe to interleave because the underlying buffers are mutex-guarded and total
// is immutable after construction.
type MemoryCollector struct {
	sample    memSampler
	total     uint64
	swapTotal uint64
	used      *ringbuffer.RingBuffer[uint64]
	cached    *ringbuffer.RingBuffer[uint64]
	free      *ringbuffer.RingBuffer[uint64]
	swapUsed  *ringbuffer.RingBuffer[uint64]
}

// NewMemoryCollector builds a collector backed by gopsutil. It takes one
// initial sample to record total physical memory and seed the buffers. It
// returns nil (after logging) when that first sample fails, since there is
// nothing useful a partially built collector could do.
func NewMemoryCollector(ctx context.Context, opts ...memOption) *MemoryCollector {
	c := &MemoryCollector{sample: defaultMemSampler}
	for _, opt := range opts {
		opt(c)
	}

	reading, err := c.sample(ctx)
	if err != nil {
		slog.Error("building memory collector", "err", err)
		return nil
	}

	c.total = reading.total
	c.swapTotal = reading.swapTotal
	c.used = ringbuffer.New[uint64](metrics.HistoryCapacity)
	c.cached = ringbuffer.New[uint64](metrics.HistoryCapacity)
	c.free = ringbuffer.New[uint64](metrics.HistoryCapacity)
	c.swapUsed = ringbuffer.New[uint64](metrics.HistoryCapacity)
	c.store(reading)
	return c
}

// Collect samples memory usage and appends the used, cached, and free byte
// counts to their buffers. It returns an error (rather than panicking) when
// sampling fails.
func (c *MemoryCollector) Collect(ctx context.Context) error {
	reading, err := c.sample(ctx)
	if err != nil {
		return err
	}
	c.store(reading)
	return nil
}

// store writes one reading to the buffers.
func (c *MemoryCollector) store(reading memReading) {
	c.used.Add(reading.used)
	c.cached.Add(reading.cached)
	c.free.Add(reading.free)
	c.swapUsed.Add(reading.swapUsed)
}

// Total returns total physical memory in bytes. It is fixed at construction.
func (c *MemoryCollector) Total() uint64 {
	return c.total
}

// SwapTotal returns total swap in bytes, fixed at construction (0 when the
// machine has no swap or it could not be read).
func (c *MemoryCollector) SwapTotal() uint64 {
	return c.swapTotal
}

// SwapUsed returns the used-swap history in bytes, oldest to newest.
func (c *MemoryCollector) SwapUsed() []uint64 {
	return c.swapUsed.Items()
}

// Used returns the used-memory history in bytes, oldest to newest.
func (c *MemoryCollector) Used() []uint64 {
	return c.used.Items()
}

// Cached returns the cached-memory history in bytes, oldest to newest.
func (c *MemoryCollector) Cached() []uint64 {
	return c.cached.Items()
}

// Free returns the free-memory history in bytes, oldest to newest.
func (c *MemoryCollector) Free() []uint64 {
	return c.free.Items()
}
