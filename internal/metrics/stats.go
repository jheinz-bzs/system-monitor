package metrics

// Float64s is a slice of samples with statistical helpers. Collectors convert a
// raw reading to this type to compute aggregate values, e.g.
// Float64s(reading).Mean() for the overall CPU percentage.
type Float64s []float64

// Sum returns the total of all values, or 0 for an empty slice.
func (f Float64s) Sum() float64 {
	var sum float64
	for _, v := range f {
		sum += v
	}
	return sum
}

// Mean returns the arithmetic mean of the values, or 0 for an empty slice.
func (f Float64s) Mean() float64 {
	if len(f) == 0 {
		return 0
	}
	return f.Sum() / float64(len(f))
}
