package monitor

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/shirou/gopsutil/v4/net"

	"github.com/josephheinz/system-monitor/internal/metrics"
	"github.com/josephheinz/system-monitor/internal/ringbuffer"
)

// netOption configures a NetworkCollector at construction. It exists so tests
// can inject a sampler and clock without a separate constructor; production
// code uses the defaults.
type netOption func(*NetworkCollector)

// withNetSampler overrides the network sampler. Tests use it to supply readings
// without real hardware.
func withNetSampler(s netSampler) netOption {
	return func(c *NetworkCollector) { c.sample = s }
}

// withNetClock overrides the clock. Tests use it to control elapsed time.
func withNetClock(now func() time.Time) netOption {
	return func(c *NetworkCollector) { c.now = now }
}

// netReading is one snapshot of the cumulative network byte counters, summed
// across all interfaces. It is the value the collector samples through so tests
// can supply readings without real hardware.
type netReading struct {
	bytesSent uint64
	bytesRecv uint64
}

// netSampler returns a single network reading. It is the seam the collector
// samples through.
type netSampler func(ctx context.Context) (netReading, error)

// defaultNetSampler reads the aggregate network I/O counters via gopsutil.
// Passing pernic=false makes gopsutil sum every interface into a single entry,
// which is exactly the upload/download/total bandwidth the Network tab charts.
func defaultNetSampler(ctx context.Context) (netReading, error) {
	counters, err := net.IOCountersWithContext(ctx, false)
	if err != nil {
		return netReading{}, fmt.Errorf("sampling network: %w", err)
	}
	if len(counters) == 0 {
		return netReading{}, fmt.Errorf("sampling network: no interface counters reported")
	}
	return netReading{
		bytesSent: counters[0].BytesSent,
		bytesRecv: counters[0].BytesRecv,
	}, nil
}

// NetworkCollector samples network throughput on each Collect and stores the
// history in ring buffers: upload, download, and total rates in bytes/sec.
// Rates are computed as the byte-counter delta divided by the elapsed
// wall-clock time since the previous sample, and total is the sum of the two.
// Reads and writes are safe to interleave because the underlying buffers are
// mutex-guarded.
type NetworkCollector struct {
	sample netSampler
	now    func() time.Time

	uploadRate   *ringbuffer.RingBuffer[uint64]
	downloadRate *ringbuffer.RingBuffer[uint64]
	totalRate    *ringbuffer.RingBuffer[uint64]

	prevSent uint64
	prevRecv uint64 // prevSent/Recv are needed because gopsutil stores BytesSent/BytesRecv as cumulative metrics
	prevTime time.Time
}

// NewNetworkCollector builds a collector backed by gopsutil. It takes one
// initial sample to record the seed counters. It returns nil (after logging)
// when that first sample fails, since there is nothing useful a partially
// built collector could do.
func NewNetworkCollector(ctx context.Context, opts ...netOption) *NetworkCollector {
	c := &NetworkCollector{sample: defaultNetSampler, now: time.Now}
	for _, opt := range opts {
		opt(c)
	}

	reading, err := c.sample(ctx)
	if err != nil {
		slog.Error("building network collector", "err", err)
		return nil
	}

	c.uploadRate = ringbuffer.New[uint64](metrics.HistoryCapacity)
	c.downloadRate = ringbuffer.New[uint64](metrics.HistoryCapacity)
	c.totalRate = ringbuffer.New[uint64](metrics.HistoryCapacity)
	c.prevSent = reading.bytesSent
	c.prevRecv = reading.bytesRecv
	c.prevTime = c.now()
	// The first sample has no prior reading to delta against, so seed the rate
	// buffers with zero (mirrors how the disk collector seeds its rates).
	c.uploadRate.Add(0)
	c.downloadRate.Add(0)
	c.totalRate.Add(0)
	return c
}

// Collect samples network throughput and appends the upload, download, and
// total byte rates (bytes/sec since the previous sample) to their buffers. It
// returns an error (rather than panicking) when sampling fails.
func (c *NetworkCollector) Collect(ctx context.Context) error {
	reading, err := c.sample(ctx)
	if err != nil {
		return err
	}

	now := c.now()
	elapsed := now.Sub(c.prevTime).Seconds()
	upload := rate(reading.bytesSent, c.prevSent, elapsed)
	download := rate(reading.bytesRecv, c.prevRecv, elapsed)
	c.uploadRate.Add(upload)
	c.downloadRate.Add(download)
	c.totalRate.Add(upload + download)

	c.prevSent = reading.bytesSent
	c.prevRecv = reading.bytesRecv
	c.prevTime = now
	return nil
}

// UploadRate returns the upload-rate history in bytes/sec, oldest to newest.
func (c *NetworkCollector) UploadRate() []uint64 {
	return c.uploadRate.Items()
}

// DownloadRate returns the download-rate history in bytes/sec, oldest to newest.
func (c *NetworkCollector) DownloadRate() []uint64 {
	return c.downloadRate.Items()
}

// TotalRate returns the total-bandwidth history (upload + download) in
// bytes/sec, oldest to newest.
func (c *NetworkCollector) TotalRate() []uint64 {
	return c.totalRate.Items()
}
