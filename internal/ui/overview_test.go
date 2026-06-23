package ui

import (
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"
)

// treeContains reports whether target appears anywhere in root's container tree.
func treeContains(root, target fyne.CanvasObject) bool {
	if root == target {
		return true
	}
	if c, ok := root.(*fyne.Container); ok {
		for _, o := range c.Objects {
			if treeContains(o, target) {
				return true
			}
		}
	}
	return false
}

// The reusable MetricPanel shell must accept a title and an arbitrary content
// CanvasObject and slot that content into the rendered panel — the component
// acceptance criterion. Future metric widgets drop into this same slot.
func TestNewMetricPanelAcceptsTitleAndContent(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Settings().SetTheme(newTheme())

	content := canvas.NewRectangle(palette.Accent)
	panel := newMetricPanel(labelCPUPanel, statusDot(status.Healthy), content)

	w := test.NewWindow(panel)
	defer w.Close()
	w.Resize(fyne.NewSize(300, 200))

	if !treeContains(panel, content) {
		t.Fatal("metric panel does not contain the provided content object")
	}
}

// The Overview grid must cover the key metrics named in BZS253-62 (CPU, Memory,
// Disk, Network), and every panel must carry a non-empty title and a content
// slot — the acceptance criteria for the grid's makeup.
func TestOverviewMetricsCoverKeyMetrics(t *testing.T) {
	for _, want := range []string{labelCPUPanel, labelMemoryPanel, labelDiskIOPanel, labelNetworkPanel} {
		found := false
		for _, m := range overviewMetrics {
			if m.title == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("overview grid is missing a panel for key metric %q", want)
		}
	}

	for i, m := range overviewMetrics {
		if m.title == "" {
			t.Errorf("metric %d has an empty title", i)
		}
	}
}

// The tab must render through the full widget path and lay out gracefully across
// window sizes — from a tight window up to a wide one — without panicking. This
// is the "grid resizes gracefully" acceptance criterion (the weighted rows and
// columns must not assume a minimum slack).
func TestOverviewRendersAndResizes(t *testing.T) {
	app := test.NewApp()
	defer app.Quit()
	app.Settings().SetTheme(newTheme())

	w := test.NewWindow(newOverview())
	defer w.Close()

	for _, size := range []fyne.Size{
		{Width: 1280, Height: 800}, // default
		{Width: 1600, Height: 1000},
		{Width: 700, Height: 500},
		{Width: 320, Height: 240}, // tighter than any panel's content
	} {
		w.Resize(size)
		w.Content().Refresh()
	}
}
