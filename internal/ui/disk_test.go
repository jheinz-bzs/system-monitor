package ui

import (
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"
)

// TestVolumesListSurvivesRendererRecreation guards the Disk tab's volumes pane
// against going permanently blank: fyne destroys renderers left unpainted for
// ~1 minute and recreates them on demand, and the recreated renderer gets no
// Layout call while the widget's size is unchanged. Arrange must therefore
// take its size from the widget (which survives recreation), not from a
// Layout-fed renderer field.
func TestVolumesListSurvivesRendererRecreation(t *testing.T) {
	test.NewApp()
	src := diskUsageSourceFunc(func() []diskPartition {
		return []diskPartition{{mount: "C:\\", total: 512, used: 420}}
	})
	list := newVolumesList(src)
	list.Resize(fyne.NewSize(400, 300))

	// A second CreateRenderer stands in for fyne recreating a destroyed
	// renderer: fresh row pool, zero internal state, only Refresh incoming.
	r := list.CreateRenderer().(*volumesListRenderer)
	r.Refresh()

	if len(r.rows) == 0 || !r.rows[0].name.Visible() {
		t.Fatal("volumes list blank after renderer recreation: no visible rows")
	}
}

func TestDiskUsageColor(t *testing.T) {
	var (
		normal   = withAlpha(palette.Accent, 0xff)
		warning  = withAlpha(palette.Yellow, 0xff)
		critical = withAlpha(palette.Red, 0xff)
	)
	cases := []struct {
		name string
		frac float64
		want color.NRGBA
	}{
		{"empty", 0.0, normal},
		{"just below warn", 0.79, normal},
		{"at warn threshold", diskWarnFraction, warning},
		{"between warn and crit", 0.89, warning},
		{"at crit threshold", diskCritFraction, critical},
		{"full", 1.0, critical},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := diskUsageColor(c.frac); got != c.want {
				t.Errorf("diskUsageColor(%v) = %v, want %v", c.frac, got, c.want)
			}
		})
	}
}

func TestDiskPercentColor(t *testing.T) {
	if got := diskPercentColor(0.5); got != palette.Text3 {
		t.Errorf("diskPercentColor(0.5) = %v, want muted text-3 %v", got, palette.Text3)
	}
	if got, want := diskPercentColor(0.85), color.Color(withAlpha(palette.Yellow, 0xff)); got != want {
		t.Errorf("diskPercentColor(0.85) = %v, want warning hue %v", got, want)
	}
}

func TestDiskTexts(t *testing.T) {
	p := diskPartition{mount: "C:\\", total: 512, used: 420}
	if got, want := diskSizeText(p), "420B / 512B"; got != want {
		t.Errorf("diskSizeText = %q, want %q", got, want)
	}
	if got, want := diskPercentText(float64(p.used)/float64(p.total)), "82% used"; got != want {
		t.Errorf("diskPercentText = %q, want %q", got, want)
	}
}

// fakeDirSource is a static diskDirSource for treemap-adapter tests.
type fakeDirSource struct {
	dirList []diskDir
}

func (f fakeDirSource) dirs() []diskDir { return f.dirList }

func TestDiskDirTreemapBlocks(t *testing.T) {
	src := diskDirTreemapSource{src: fakeDirSource{
		dirList: []diskDir{
			{label: "Users", path: "C:\\Users", bytes: 300},
			{label: "Apps", path: "C:\\Apps", bytes: 50},
		},
	}}
	blocks := src.treemapBlocks()

	// One block per directory, order preserved (the snapshot arrives sorted),
	// each with a distinct categorical hue by position and the full path as its
	// hover tooltip. No free tile.
	wantLabels := []string{"Users", "Apps"}
	wantWeights := []float64{300, 50}
	wantTips := []string{"C:\\Users", "C:\\Apps"}
	for i := range wantLabels {
		if blocks[i].label != wantLabels[i] {
			t.Errorf("block[%d].label = %q, want %q", i, blocks[i].label, wantLabels[i])
		}
		if blocks[i].weight != wantWeights[i] {
			t.Errorf("block[%d].weight = %v, want %v", i, blocks[i].weight, wantWeights[i])
		}
		if blocks[i].tooltip != wantTips[i] {
			t.Errorf("block[%d].tooltip = %q, want %q", i, blocks[i].tooltip, wantTips[i])
		}
		if want := palette.Series[i%len(palette.Series)]; blocks[i].fill != want {
			t.Errorf("block[%d].fill = %v, want categorical %v", i, blocks[i].fill, want)
		}
	}
}

func TestDiskSubtitle(t *testing.T) {
	parts := []diskPartition{
		{mount: "C:\\", total: 512, used: 420},
		{mount: "pseudo", total: 0, used: 0}, // no capacity → excluded from count and total
		{mount: "D:\\", total: 1000, used: 950},
	}
	// 1512 bytes summed → "1.5K"; two real volumes.
	if got, want := diskSubtitle(parts), "2 volumes · 1.5K total"; got != want {
		t.Errorf("diskSubtitle = %q, want %q", got, want)
	}
}
