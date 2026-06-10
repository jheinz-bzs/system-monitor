package monitor

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/shirou/gopsutil/v4/cpu"

	"github.com/josephheinz/system-monitor/internal/metrics"
	"github.com/josephheinz/system-monitor/internal/ringbuffer"
)

// CPUSummary is a static description of the processor: the logical core count
// and the marketing model name. It feeds the CPU tab's page header.
type CPUSummary struct {
	Cores     int
	ModelName string
}

// CPUInfo reads the processor description once via gopsutil. It is static data,
// so callers fetch it at startup rather than through a polled collector.
func CPUInfo(ctx context.Context) (CPUSummary, error) {
	cores, err := cpu.CountsWithContext(ctx, true)
	if err != nil {
		return CPUSummary{}, fmt.Errorf("counting logical cores: %w", err)
	}
	infos, err := cpu.InfoWithContext(ctx)
	if err != nil {
		return CPUSummary{}, fmt.Errorf("reading cpu info: %w", err)
	}
	summary := CPUSummary{Cores: cores}
	if len(infos) > 0 {
		summary.ModelName = infos[0].ModelName
	}
	return summary, nil
}

// coreSampler returns the per-logical-core CPU usage percentages (0..100), one
// value per logical core. It is the seam the collector samples through so tests
// can supply readings without real hardware.
type coreSampler func(ctx context.Context) ([]float64, error)

// cpuOption configures a CPUCollector at construction. It exists so tests can
// inject a sampler without a separate constructor; production code uses the
// defaults.
type cpuOption func(*CPUCollector)

// withCPUSampler overrides the CPU sampler. Tests use it to supply readings
// without real hardware.
func withCPUSampler(s coreSampler) cpuOption {
	return func(c *CPUCollector) { c.sample = s }
}

// defaultSampler reads per-core usage via gopsutil. An interval of 0 measures
// usage since the previous call, so it never blocks the ticker.
func defaultSampler(ctx context.Context) ([]float64, error) {
	reading, err := cpu.PercentWithContext(ctx, 0, true)
	if err != nil {
		return nil, fmt.Errorf("sampling cpu: %w", err)
	}
	return reading, nil
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
// sample to learn the logical core count and seed the buffers. It returns nil
// (after logging) when that first sample fails or reports no cores, since
// there is nothing useful a partially built collector could do.
func NewCPUCollector(ctx context.Context, opts ...cpuOption) *CPUCollector {
	c := &CPUCollector{sample: defaultSampler}
	for _, opt := range opts {
		opt(c)
	}

	reading, err := c.sample(ctx)
	if err != nil {
		slog.Error("building cpu collector", "err", err)
		return nil
	}
	if len(reading) == 0 {
		slog.Error("building cpu collector", "err", "no logical cores reported")
		return nil
	}

	c.overall = ringbuffer.New[float64](metrics.HistoryCapacity)
	c.perCore = make([]*ringbuffer.RingBuffer[float64], len(reading))
	for i := range c.perCore {
		c.perCore[i] = ringbuffer.New[float64](metrics.HistoryCapacity)
	}
	c.store(reading)
	return c
}

// Collect samples CPU usage and appends the overall percentage and each core's
// percentage to their buffers. It returns an error (rather than panicking) when
// sampling fails or the reported core count changes.
func (c *CPUCollector) Collect(ctx context.Context) error {
	reading, err := c.sample(ctx)
	if err != nil {
		return err
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
	c.overall.Add(metrics.Float64s(reading).Mean())
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
