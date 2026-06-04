package monitor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/disk"

	"github.com/josephheinz/system-monitor/internal/metrics"
	"github.com/josephheinz/system-monitor/internal/ringbuffer"
)

// PartitionUsage is storage usage for one mounted partition, in bytes.
type PartitionUsage struct {
	Mountpoint string
	Fstype     string
	Total      uint64
	Used       uint64
}

// diskReading is one snapshot of disk state: per-partition usage plus the
// cumulative I/O byte counters summed across all disks. It is the value the
// collector samples through so tests can supply readings without real hardware.
type diskReading struct {
	partitions []PartitionUsage
	readBytes  uint64
	writeBytes uint64
}

// diskSampler returns a single disk reading. It is the seam the collector
// samples through.
type diskSampler func(ctx context.Context) (diskReading, error)

// defaultDiskSampler reads partition usage and I/O counters via gopsutil. It
// samples every mounted partition (including pseudo-filesystems). Usage failures
// on individual mounts are skipped rather than failing the whole sample, because
// unreadable or permission-denied mounts are routine on a real machine; a
// failure to enumerate partitions or read I/O counters is returned as an error.
func defaultDiskSampler(ctx context.Context) (diskReading, error) {
	parts, err := disk.PartitionsWithContext(ctx, true)
	if err != nil {
		return diskReading{}, fmt.Errorf("listing partitions: %w", err)
	}

	usage := make([]PartitionUsage, 0, len(parts))
	for _, p := range parts {
		stat, err := disk.UsageWithContext(ctx, p.Mountpoint)
		if err != nil {
			continue // unreadable mount; skip it rather than failing the sample
		}
		usage = append(usage, PartitionUsage{
			Mountpoint: p.Mountpoint,
			Fstype:     p.Fstype,
			Total:      stat.Total,
			Used:       stat.Used,
		})
	}

	counters, err := disk.IOCountersWithContext(ctx)
	if err != nil {
		return diskReading{}, fmt.Errorf("reading io counters: %w", err)
	}
	var readBytes, writeBytes uint64
	for _, c := range counters {
		readBytes += c.ReadBytes
		writeBytes += c.WriteBytes
	}

	return diskReading{partitions: usage, readBytes: readBytes, writeBytes: writeBytes}, nil
}

// DiskCollector samples disk state on each Collect. Storage usage is held as a
// snapshot slice (it changes slowly and is consumed per-partition, not as a time
// series), while read and write I/O rates in bytes/sec are stored in ring
// buffers. Rates are computed as the byte-counter delta divided by the elapsed
// wall-clock time since the previous sample. Reads and writes are safe to
// interleave: the buffers are mutex-guarded and the usage snapshot is guarded by
// its own RWMutex.
type DiskCollector struct {
	sample diskSampler
	now    func() time.Time

	mu    sync.RWMutex
	usage []PartitionUsage

	readRate  *ringbuffer.RingBuffer[uint64]
	writeRate *ringbuffer.RingBuffer[uint64]

	prevRead  uint64
	prevWrite uint64 // prevRead/Write are needed because gopsutil stores ReadBytes/WriteBytes as a cumulative metric
	prevTime  time.Time
}

// NewDiskCollector builds a collector backed by gopsutil. It takes one initial
// sample to record the seed usage snapshot and I/O counters.
func NewDiskCollector(ctx context.Context) (*DiskCollector, error) {
	return newDiskCollector(ctx, defaultDiskSampler, time.Now)
}

// newDiskCollector is the testable constructor: it samples through the given
// seam and reads the clock through now, so tests can control both readings and
// elapsed time.
func newDiskCollector(ctx context.Context, sample diskSampler, now func() time.Time) (*DiskCollector, error) {
	reading, err := sample(ctx)
	if err != nil {
		return nil, fmt.Errorf("sampling disk: %w", err)
	}

	c := &DiskCollector{
		sample:    sample,
		now:       now,
		usage:     reading.partitions,
		readRate:  ringbuffer.New[uint64](metrics.HistoryCapacity),
		writeRate: ringbuffer.New[uint64](metrics.HistoryCapacity),
		prevRead:  reading.readBytes,
		prevWrite: reading.writeBytes,
		prevTime:  now(),
	}
	// The first sample has no prior reading to delta against, so seed the rate
	// buffers with zero (mirrors how the CPU/memory collectors seed from their
	// first reading).
	c.readRate.Add(0)
	c.writeRate.Add(0)
	return c, nil
}

// Collect samples disk state, replaces the usage snapshot, and appends the read
// and write byte rates (bytes/sec since the previous sample) to their buffers.
// It returns an error (rather than panicking) when sampling fails.
func (c *DiskCollector) Collect(ctx context.Context) error {
	reading, err := c.sample(ctx)
	if err != nil {
		return fmt.Errorf("sampling disk: %w", err)
	}

	c.mu.Lock()
	c.usage = reading.partitions
	c.mu.Unlock()

	now := c.now()
	elapsed := now.Sub(c.prevTime).Seconds()
	readRate := rate(reading.readBytes, c.prevRead, elapsed)
	writeRate := rate(reading.writeBytes, c.prevWrite, elapsed)
	c.readRate.Add(readRate)
	c.writeRate.Add(writeRate)

	c.prevRead = reading.readBytes
	c.prevWrite = reading.writeBytes
	c.prevTime = now
	return nil
}

// rate returns the per-second byte rate from a cumulative counter delta. It
// returns 0 when no time has elapsed or when the counter went backwards (a reset
// or overflow), so a wrap never produces a spurious spike.
func rate(cur, prev uint64, elapsedSeconds float64) uint64 {
	if elapsedSeconds <= 0 || cur < prev {
		return 0
	}
	return uint64(float64(cur-prev) / elapsedSeconds)
}

// Usage returns a copy of the latest per-partition usage snapshot.
func (c *DiskCollector) Usage() []PartitionUsage {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]PartitionUsage, len(c.usage))
	copy(out, c.usage)
	return out
}

// ReadRate returns the read-rate history in bytes/sec, oldest to newest.
func (c *DiskCollector) ReadRate() []uint64 {
	return c.readRate.Items()
}

// WriteRate returns the write-rate history in bytes/sec, oldest to newest.
func (c *DiskCollector) WriteRate() []uint64 {
	return c.writeRate.Items()
}
