package ringbuffer

import (
	"reflect"
	"sync"
	"testing"
)

func TestNewPanicsOnNonPositiveCapacity(t *testing.T) {
	for _, capacity := range []int{0, -1} {
		t.Run("", func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatalf("New(%d) did not panic", capacity)
				}
			}()
			New[int](capacity)
		})
	}
}

func TestEmptyBuffer(t *testing.T) {
	r := New[int](3)

	if got := r.Len(); got != 0 {
		t.Errorf("Len() = %d, want 0", got)
	}
	if got := r.Cap(); got != 3 {
		t.Errorf("Cap() = %d, want 3", got)
	}
	if got := r.Items(); len(got) != 0 {
		t.Errorf("Items() = %v, want empty", got)
	}
	if got := r.Latest(); got != nil {
		t.Errorf("Latest() = %v on empty buffer, want nil", got)
	}
}

func TestWritesBelowCapacity(t *testing.T) {
	r := New[int](5)
	r.Add(1)
	r.Add(2)
	r.Add(3)

	if got := r.Len(); got != 3 {
		t.Errorf("Len() = %d, want 3", got)
	}
	if want := []int{1, 2, 3}; !reflect.DeepEqual(r.Items(), want) {
		t.Errorf("Items() = %v, want %v", r.Items(), want)
	}
	if v := r.Latest(); v == nil || *v != 3 {
		t.Errorf("Latest() = %v, want 3", v)
	}
}

func TestFullBuffer(t *testing.T) {
	r := New[int](3)
	r.Add(1)
	r.Add(2)
	r.Add(3)

	if got := r.Len(); got != 3 {
		t.Errorf("Len() = %d, want 3", got)
	}
	if want := []int{1, 2, 3}; !reflect.DeepEqual(r.Items(), want) {
		t.Errorf("Items() = %v, want %v", r.Items(), want)
	}
}

func TestOverwriteOldest(t *testing.T) {
	r := New[int](3)
	for _, v := range []int{1, 2, 3, 4, 5} {
		r.Add(v)
	}

	if got := r.Len(); got != 3 {
		t.Errorf("Len() = %d, want 3 (must not exceed Cap)", got)
	}
	if want := []int{3, 4, 5}; !reflect.DeepEqual(r.Items(), want) {
		t.Errorf("Items() = %v, want %v", r.Items(), want)
	}
	if v := r.Latest(); v == nil || *v != 5 {
		t.Errorf("Latest() = %v, want 5", v)
	}
}

func TestItemsReturnsCopy(t *testing.T) {
	r := New[int](3)
	r.Add(1)
	r.Add(2)

	items := r.Items()
	items[0] = 99

	if want := []int{1, 2}; !reflect.DeepEqual(r.Items(), want) {
		t.Errorf("Items() = %v after mutating returned slice, want %v", r.Items(), want)
	}
}

func TestStructElementType(t *testing.T) {
	type sample struct {
		ts  int64
		val float64
	}
	r := New[sample](2)
	r.Add(sample{ts: 1, val: 1.5})
	r.Add(sample{ts: 2, val: 2.5})
	r.Add(sample{ts: 3, val: 3.5})

	want := []sample{{ts: 2, val: 2.5}, {ts: 3, val: 3.5}}
	if !reflect.DeepEqual(r.Items(), want) {
		t.Errorf("Items() = %v, want %v", r.Items(), want)
	}
}

func TestConcurrentAccess(t *testing.T) {
	r := New[int](64)
	var wg sync.WaitGroup

	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				r.Add(i)
			}
		}()
	}
	for reader := 0; reader < 4; reader++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 1000; i++ {
				_ = r.Items()
				_ = r.Latest()
				_ = r.Len()
			}
		}()
	}

	wg.Wait()

	if got := r.Len(); got != r.Cap() {
		t.Errorf("Len() = %d after saturating writes, want %d", got, r.Cap())
	}
}
