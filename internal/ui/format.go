package ui

// Chart math and formatting helpers extracted from linechart.go (SRP: these
// are pure numeric / time functions with no Fyne dependency and a single reason
// to change — the chart's labeling and scaling logic).

import (
	"math"
	"strconv"
	"time"
)

// chartBox is the pixel rectangle of the plot area (the framed region, excluding
// the label gutters).
type chartBox struct {
	x, y, width, height float32
}

// valueToY maps a data value onto a pixel Y within the plot box, with hi at the
// top edge and lo at the bottom. Values are clamped to the range so a fixed
// axis (e.g. 0–100) can't draw outside the frame.
func valueToY(v, lo, hi float64, plot chartBox) float32 {
	if hi == lo {
		return plot.y + plot.height
	}
	frac := (v - lo) / (hi - lo)
	frac = math.Max(0, math.Min(1, frac))
	return plot.y + plot.height*float32(1-frac)
}

// tickValues returns the n tick values from top (hi) to bottom (lo), evenly
// spaced — matching the top→bottom order of the Y label pool.
func tickValues(lo, hi float64, n int) []float64 {
	out := make([]float64, n)
	for i := 0; i < n; i++ {
		frac := float64(i) / float64(n-1)
		out[i] = hi - frac*(hi-lo)
	}
	return out
}

// niceRange expands [min, max] to bounds that fall on "nice" round numbers,
// giving readable tick labels for an auto-scaled axis. Adapted from the classic
// Graphics Gems labeling algorithm.
func niceRange(min, max float64, ticks int) (lo, hi float64) {
	if min == max {
		// Flat series: pad to a unit so the line sits mid-plot.
		min -= 0.5
		max += 0.5
	}
	step := niceNum((max-min)/float64(ticks), true)
	lo = math.Floor(min/step) * step
	hi = math.Ceil(max/step) * step
	return lo, hi
}

// niceNum returns a "nice" number near x: rounded to 1/2/5×10ⁿ when round is
// set, otherwise the next-larger such number.
func niceNum(x float64, round bool) float64 {
	if x <= 0 {
		return 1
	}
	exp := math.Floor(math.Log10(x))
	f := x / math.Pow(10, exp)
	var nf float64
	switch {
	case round && f < 1.5, !round && f <= 1:
		nf = 1
	case round && f < 3, !round && f <= 2:
		nf = 2
	case round && f < 7, !round && f <= 5:
		nf = 5
	default:
		nf = 10
	}
	return nf * math.Pow(10, exp)
}

// formatCompact is the default Y-label formatter: a trimmed decimal.
func formatCompact(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// percentMax is the top of the percentage domain shared by the CPU charts'
// fixed Y axis and the process bars' fill fraction.
const percentMax = 100

// Byte-quantity formatting for formatBytesShort.
const (
	bytesPerStep = 1024 // binary unit step between suffixes
	// Below this a scaled value keeps one decimal ("1.8G"); at or above it the
	// decimal is dropped ("612M") — the wireframe table's compact style.
	byteDecimalLimit = 10
)

// byteSuffixes are the unit suffixes for formatBytesShort, in ascending
// bytesPerStep powers.
var byteSuffixes = []string{"B", "K", "M", "G", "T", "P"}

// formatBytesShort renders a byte count in the data table's compact style:
// "88M", "612M", "1.8G".
func formatBytesShort(b uint64) string {
	v := float64(b)
	step := 0
	for v >= bytesPerStep && step < len(byteSuffixes)-1 {
		v /= bytesPerStep
		step++
	}
	if step > 0 && v < byteDecimalLimit {
		return strconv.FormatFloat(v, 'f', 1, 64) + byteSuffixes[step]
	}
	return strconv.FormatFloat(v, 'f', 0, 64) + byteSuffixes[step]
}

// formatSpan renders a history window for panel titles: "45 s" under a minute,
// otherwise "1 min".
func formatSpan(d time.Duration) string {
	if d < time.Minute {
		return strconv.Itoa(int(d.Round(time.Second)/time.Second)) + " s"
	}
	return strconv.Itoa(int(d.Round(time.Minute)/time.Minute)) + " min"
}

// formatAge renders an elapsed time for the X axis: "now" at zero, "−Ns" under
// a minute, otherwise "−Mm".
func formatAge(d time.Duration) string {
	if d <= 0 {
		return "now"
	}
	if d < time.Minute {
		return "-" + strconv.Itoa(int(d.Round(time.Second)/time.Second)) + "s"
	}
	return "-" + strconv.Itoa(int(d.Round(time.Minute)/time.Minute)) + "m"
}

// clamp32 constrains v to [lo, hi].
func clamp32(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
