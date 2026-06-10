// Package series defines the neutral data seam between the monitor collectors
// and the UI chart widgets. Neither side imports the other; both depend only on
// this package.
//
// The seam is intentionally minimal: Source is a single-method interface
// (ISP-clean), SourceFunc adapts any func()[]float64, and SourceFrom adapts any
// numeric ring buffer — the generic constraint lets the chart work with
// float64, uint64, etc. without an explicit conversion at every call site.
package series

// Numeric is the set of element kinds a metric ring buffer can hold. It mirrors
// the constraint the chart's SourceFrom adapter accepts.
type Numeric interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 |
		~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr |
		~float32 | ~float64
}

// Source yields a series' samples ordered oldest → newest (the same order
// RingBuffer.Items returns), which the chart plots left → right.
type Source interface {
	Values() []float64
}

// SourceFunc adapts a plain func to a Source.
type SourceFunc func() []float64

func (f SourceFunc) Values() []float64 { return f() }

// SourceFrom adapts any numeric ring buffer — anything exposing Items() []T for
// a Numeric T — into a Source, converting samples to float64. The buffer is
// re-read on every Values() call so the chart always reflects the latest window.
//
// Type inference usually picks T up from the argument; pass it explicitly if a
// call site is ambiguous, e.g. SourceFrom[uint64](rxBuf).
func SourceFrom[T Numeric](buf interface{ Items() []T }) Source {
	return SourceOf(buf.Items)
}

// SourceOf adapts a snapshot func returning numeric samples into a Source,
// converting them to float64. It serves collectors that expose history through
// methods rather than a bare ring buffer (e.g. MemoryCollector.Used). The func
// is re-invoked on every Values() call so the chart always reflects the latest
// window.
func SourceOf[T Numeric](snap func() []T) Source {
	return SourceFunc(func() []float64 {
		in := snap()
		out := make([]float64, len(in))
		for i, v := range in {
			out[i] = float64(v)
		}
		return out
	})
}
