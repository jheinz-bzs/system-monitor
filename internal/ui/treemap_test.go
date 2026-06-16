package ui

import (
	"slices"
	"testing"

	"fyne.io/fyne/v2"
)

// blockLabels returns a treemap source's block labels in order — the laid-out
// order the squarified widget receives.
func blockLabels(s *processTreemapSource) []string {
	blocks := s.treemapBlocks()
	out := make([]string, len(blocks))
	for i, b := range blocks {
		out[i] = b.label
	}
	return out
}

func TestTreemapBlocksDefaultToCPUDescending(t *testing.T) {
	s := newProcessTreemapSource(fixedProcs(testRows()))

	// CPU: charlie/Bravo tie at 30, alpha 5. sortRows breaks the tie by
	// ascending PID, so Bravo (20) precedes charlie (30).
	want := []string{"Bravo", "charlie", "alpha"}
	if got := blockLabels(s); !slices.Equal(got, want) {
		t.Errorf("CPU blocks = %v, want %v", got, want)
	}
}

func TestTreemapBlocksByMemoryDescending(t *testing.T) {
	s := newProcessTreemapSource(fixedProcs(testRows()))
	s.setMetric(treemapMetricMem)

	// Memory: charlie 200, alpha 100, Bravo 50.
	want := []string{"charlie", "alpha", "Bravo"}
	if got := blockLabels(s); !slices.Equal(got, want) {
		t.Errorf("memory blocks = %v, want %v", got, want)
	}
}

func TestTreemapBlocksDropZeroWeight(t *testing.T) {
	rows := []processRow{
		{pid: 1, name: "busy", cpu: 40, mem: 10},
		{pid: 2, name: "idle", cpu: 0, mem: 10},
	}
	s := newProcessTreemapSource(fixedProcs(rows))

	want := []string{"busy"}
	if got := blockLabels(s); !slices.Equal(got, want) {
		t.Errorf("CPU blocks = %v, want %v (a 0%% process gets no block)", got, want)
	}
}

func TestTreemapBlocksCapAtLimit(t *testing.T) {
	rows := make([]processRow, treemapBlockLimit+10)
	for i := range rows {
		// Descending CPU so the top treemapBlockLimit survive the cap.
		rows[i] = processRow{pid: PID(i + 1), name: "p", cpu: float64(len(rows) - i)}
	}
	s := newProcessTreemapSource(fixedProcs(rows))

	if got := len(s.treemapBlocks()); got != treemapBlockLimit {
		t.Errorf("block count = %d, want %d (capped at the block limit)", got, treemapBlockLimit)
	}
}

func TestTreemapPidAtMapsBlockIndexToProcess(t *testing.T) {
	s := newProcessTreemapSource(fixedProcs(testRows()))
	s.treemapBlocks() // records the PID-per-block mapping a tap resolves against

	// CPU desc, PID tiebreak: Bravo(20), charlie(30), alpha(10).
	want := []PID{20, 30, 10}
	for i, w := range want {
		if got, ok := s.pidAt(i); !ok || got != w {
			t.Errorf("pidAt(%d) = %d, %t; want %d, true", i, got, ok, w)
		}
	}
	if _, ok := s.pidAt(len(want)); ok {
		t.Errorf("pidAt past the last block reported ok; want false")
	}
	if _, ok := s.pidAt(-1); ok {
		t.Errorf("pidAt(-1) reported ok; want false")
	}
}

func TestTreemapHitAt(t *testing.T) {
	tm := &treemap{hits: []treemapHit{
		{x: 0, y: 0, w: 10, h: 10, index: 0, tooltip: "a"},
		{x: 20, y: 0, w: 10, h: 10, index: 1, tooltip: "b"}, // 10px gutter at x∈[10,20)
	}}

	if h := tm.hitAt(fyne.NewPos(5, 5)); h == nil || h.index != 0 {
		t.Errorf("hitAt inside block 0 = %v, want index 0", h)
	}
	if h := tm.hitAt(fyne.NewPos(15, 5)); h != nil {
		t.Errorf("hitAt on the gutter = %v, want nil", h)
	}
	if h := tm.hitAt(fyne.NewPos(25, 5)); h == nil || h.index != 1 {
		t.Errorf("hitAt inside block 1 = %v, want index 1", h)
	}
}

func TestTreemapHoverTooltip(t *testing.T) {
	tm := &treemap{hoverIndex: noTreemapHover, hits: []treemapHit{{index: 0, tooltip: "C:\\Users"}}}
	r := &treemapRenderer{tm: tm}

	if got := r.hoverTooltip(); got != "" {
		t.Errorf("no hover → tooltip %q, want empty", got)
	}
	tm.hoverIndex = 0
	if got := r.hoverTooltip(); got != "C:\\Users" {
		t.Errorf("hovering block 0 → tooltip %q, want %q", got, "C:\\Users")
	}
	tm.hoverIndex = 5 // an index not currently drawn (data changed under the cursor)
	if got := r.hoverTooltip(); got != "" {
		t.Errorf("hovering an undrawn index → tooltip %q, want empty", got)
	}
}

func TestClampTip(t *testing.T) {
	cases := []struct {
		name               string
		v, size, max, want float32
	}{
		{"fits", 10, 30, 100, 10},
		{"overflow pushes left to fit", 90, 30, 100, 70},
		{"too wide clamps to zero", 90, 200, 100, 0},
	}
	for _, c := range cases {
		if got := clampTip(c.v, c.size, c.max); got != c.want {
			t.Errorf("%s: clampTip(%v,%v,%v) = %v, want %v", c.name, c.v, c.size, c.max, got, c.want)
		}
	}
}

func TestTreemapBlocksColorWrapsCategorically(t *testing.T) {
	rows := make([]processRow, len(palette.Series)+1)
	for i := range rows {
		rows[i] = processRow{pid: PID(i + 1), name: "p", cpu: float64(len(rows) - i)}
	}
	blocks := newProcessTreemapSource(fixedProcs(rows)).treemapBlocks()

	// The (n+1)th block wraps back to the first categorical hue.
	if blocks[len(palette.Series)].fill != palette.Series[0] {
		t.Errorf("color did not wrap after %d series", len(palette.Series))
	}
}
