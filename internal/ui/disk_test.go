package ui

import (
	"image/color"
	"testing"
)

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
