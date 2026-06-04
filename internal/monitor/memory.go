package monitor

import (
	"context"
	"fmt"

	"github.com/shirou/gopsutil/v4/mem"

	"github.com/josephheinz/system-monitor/internal/metrics"
	"github.com/josephheinz/system-monitor/internal/ringbuffer"
)

// memReading is one snapshot of memory in bytes. It is the value the collector
// samples through so tests can supply readings without real hardware.
type memReading struct {
	total  uint64
	used   uint64
	cached uint64
	free   uint64
}

// memSampler returns a single memory reading. It is the seam the collector
// samples through.
type memSampler func(ctx context.Context) (memReading, error)

// defaultMemSampler reads virtual memory stats via gopsutil.
func defaultMemSampler(ctx context.Context) (memReading, error) {
	stat, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return memReading{}, fmt.Errorf("sampling memory: %w", err)
	}
	return memReading{
		total:  stat.Total,
		used:   stat.Used,
		cached: stat.Cached,
		free:   stat.Free,
	}, nil
}

// MemoryCollector samples memory usage on each Collect and stores the history
// in ring buffers: one each for used, cached, and free bytes. Total physical
// memory is a static field because it does not change. Reads and writes are
// safe to interleave because the underlying buffers are mutex-guarded and total
// is immutable after construction.
type MemoryCollector struct {
	sample memSampler
	total  uint64
	used   *ringbuffer.RingBuffer[uint64]
	cached *ringbuffer.RingBuffer[uint64]
	free   *ringbuffer.RingBuffer[uint64]
}

// NewMemoryCollector builds a collector backed by gopsutil. It takes one
// initial sample to record total physical memory and seed the buffers.
func NewMemoryCollector(ctx context.Context) (*MemoryCollector, error) {
	return newMemoryCollector(ctx, defaultMemSampler)
}

// newMemoryCollector is the testable constructor: it samples through the given
// seam to record total memory and seed the buffers.
func newMemoryCollector(ctx context.Context, sample memSampler) (*MemoryCollector, error) {
	reading, err := sample(ctx)
	if err != nil {
		return nil, fmt.Errorf("sampling memory: %w", err)
	}

	c := &MemoryCollector{
		sample: sample,
		total:  reading.total,
		used:   ringbuffer.New[uint64](metrics.HistoryCapacity),
		cached: ringbuffer.New[uint64](metrics.HistoryCapacity),
		free:   ringbuffer.New[uint64](metrics.HistoryCapacity),
	}
	c.store(reading)
	return c, nil
}

// Collect samples memory usage and appends the used, cached, and free byte
// counts to their buffers. It returns an error (rather than panicking) when
// sampling fails.
func (c *MemoryCollector) Collect(ctx context.Context) error {
	reading, err := c.sample(ctx)
	if err != nil {
		return fmt.Errorf("sampling memory: %w", err)
	}
	c.store(reading)
	return nil
}

// store writes one reading to the buffers.
func (c *MemoryCollector) store(reading memReading) {
	c.used.Add(reading.used)
	c.cached.Add(reading.cached)
	c.free.Add(reading.free)
}

// Total returns total physical memory in bytes. It is fixed at construction.
func (c *MemoryCollector) Total() uint64 {
	return c.total
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
