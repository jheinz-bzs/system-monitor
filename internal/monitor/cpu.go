package monitor

import (
	"context"
	"fmt"

	"github.com/shirou/gopsutil/v4/cpu"

	"github.com/josephheinz/system-monitor/internal/metrics"
	"github.com/josephheinz/system-monitor/internal/ringbuffer"
)

// coreSampler returns the per-logical-core CPU usage percentages (0..100), one
// value per logical core. It is the seam the collector samples through so tests
// can supply readings without real hardware.
type coreSampler func(ctx context.Context) ([]float64, error)

// defaultSampler reads per-core usage via gopsutil. An interval of 0 measures
// usage since the previous call, so it never blocks the ticker.
func defaultSampler(ctx context.Context) ([]float64, error) {
	return cpu.PercentWithContext(ctx, 0, true)
}

// CPUCollector samples CPU usage on each Collect and stores the history in ring
// buffers: one for the overall percentage and one per logical core. Reads and
// writes are safe to interleave because the underlying buffers are mutex-guarded
// and the per-core slice is fixed after construction.
type CPUCollector struct {
	sample  coreSampler
	overall *ringbuffer.RingBuffer[float64]
	perCore []*ringbuffer.RingBuffer[float64]
}

// NewCPUCollector builds a collector backed by gopsutil. It takes one initial
// sample to learn the logical core count and seed the buffers.
func NewCPUCollector(ctx context.Context) (*CPUCollector, error) {
	return newCPUCollector(ctx, defaultSampler)
}

// newCPUCollector is the testable constructor: it samples through the given
// seam to size the per-core buffers from the reading's length.
func newCPUCollector(ctx context.Context, sample coreSampler) (*CPUCollector, error) {
	reading, err := sample(ctx)
	if err != nil {
		return nil, fmt.Errorf("sampling cpu: %w", err)
	}
	if len(reading) == 0 {
		return nil, fmt.Errorf("sampling cpu: no logical cores reported")
	}

	perCore := make([]*ringbuffer.RingBuffer[float64], len(reading))
	for i := range perCore {
		perCore[i] = ringbuffer.New[float64](metrics.HistoryCapacity)
	}

	c := &CPUCollector{
		sample:  sample,
		overall: ringbuffer.New[float64](metrics.HistoryCapacity),
		perCore: perCore,
	}
	c.store(reading)
	return c, nil
}

// Collect samples CPU usage and appends the overall percentage and each core's
// percentage to their buffers. It returns an error (rather than panicking) when
// sampling fails or the reported core count changes.
func (c *CPUCollector) Collect(ctx context.Context) error {
	reading, err := c.sample(ctx)
	if err != nil {
		return fmt.Errorf("sampling cpu: %w", err)
	}
	if len(reading) != len(c.perCore) {
		return fmt.Errorf("sampling cpu: got %d cores, want %d", len(reading), len(c.perCore))
	}
	c.store(reading)
	return nil
}

// store writes one per-core reading to the buffers. The caller guarantees the
// reading length matches the per-core buffer count.
func (c *CPUCollector) store(reading []float64) {
	for i, v := range reading {
		c.perCore[i].Add(v)
	}
	c.overall.Add(meanFloat64(reading))
}

// Overall returns the overall CPU usage history, oldest to newest.
func (c *CPUCollector) Overall() []float64 {
	return c.overall.Items()
}

// PerCore returns each logical core's usage history, oldest to newest, indexed
// by core.
func (c *CPUCollector) PerCore() [][]float64 {
	out := make([][]float64, len(c.perCore))
	for i, buf := range c.perCore {
		out[i] = buf.Items()
	}
	return out
}

// CoreCount returns the number of logical cores being tracked.
func (c *CPUCollector) CoreCount() int {
	return len(c.perCore)
}

// meanFloat64 returns the arithmetic mean of values, or 0 for an empty slice.
func meanFloat64(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum float64
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}
