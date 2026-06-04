// Package ringbuffer provides a generic, thread-safe fixed-capacity ring
// buffer. It is the shared storage primitive for metric history: a polling
// ticker writes samples while the UI reads them concurrently, and the buffer
// never grows beyond the capacity set at construction.
package ringbuffer

import "sync"

// RingBuffer is a fixed-capacity circular buffer of T. Once full, the oldest
// value is overwritten on the next write. All methods are safe for concurrent
// use.
type RingBuffer[T any] struct {
	mu    sync.RWMutex
	buf   []T
	head  int // index of the next write
	count int // items currently stored (<= cap)
}

// New returns an empty RingBuffer that holds up to capacity items. It panics
// when capacity is not positive: a misconfigured buffer is an unrecoverable
// startup error, not a runtime condition callers can handle.
func New[T any](capacity int) *RingBuffer[T] {
	if capacity <= 0 {
		panic("ringbuffer: capacity must be positive")
	}
	return &RingBuffer[T]{buf: make([]T, capacity)}
}

// Add appends v as the newest value, overwriting the oldest when the buffer is
// full.
func (r *RingBuffer[T]) Add(v T) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.buf[r.head] = v
	r.head = (r.head + 1) % len(r.buf)
	if r.count < len(r.buf) {
		r.count++
	}
}

// Items returns a fresh slice of the stored values from oldest to newest. The
// returned slice is a copy; mutating it does not affect the buffer.
func (r *RingBuffer[T]) Items() []T {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]T, r.count)
	start := r.oldestIndex()
	for i := 0; i < r.count; i++ {
		out[i] = r.buf[(start+i)%len(r.buf)]
	}
	return out
}

// Latest returns a pointer to the most recently added value, or nil when the
// buffer is empty. The pointed-to value is a copy; mutating it does not affect
// the buffer.
func (r *RingBuffer[T]) Latest() *T {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.count == 0 {
		return nil
	}
	newest := (r.head - 1 + len(r.buf)) % len(r.buf)
	v := r.buf[newest]
	return &v
}

// Len reports the number of items currently stored, never more than Cap.
func (r *RingBuffer[T]) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.count
}

// Cap reports the fixed capacity set at construction.
func (r *RingBuffer[T]) Cap() int {
	return len(r.buf)
}

// oldestIndex returns the backing-slice index of the oldest stored value. The
// caller must hold the lock.
func (r *RingBuffer[T]) oldestIndex() int {
	if r.count < len(r.buf) {
		return 0
	}
	return r.head
}
