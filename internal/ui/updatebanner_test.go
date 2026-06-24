package ui

import (
	"testing"

	"github.com/josephheinz/system-monitor/internal/update"
)

// TestUpdateBannerVisibility locks the banner's show/hide branching: hidden when
// idle, shown (with the version spliced in) when an update is available, and
// stuck hidden once dismissed even while the update is still available.
func TestUpdateBannerVisibility(t *testing.T) {
	state := update.StatusIdle
	src := buildSources{
		updateStatus: func() update.Snapshot { return update.Snapshot{State: state, NewVersion: "v9.9.9"} },
		startUpdate:  func() {},
	}
	b := newUpdateBannerView(src)

	b.refresh()
	if b.root.Visible() {
		t.Error("banner visible while idle")
	}

	state = update.StatusAvailable
	b.refresh()
	if !b.root.Visible() {
		t.Fatal("banner hidden while update available")
	}
	if want := labelUpdateBannerPrefix + "v9.9.9" + labelUpdateBannerSuffix; b.text.Text != want {
		t.Errorf("banner text = %q, want %q", b.text.Text, want)
	}

	b.dismiss()
	b.refresh()
	if b.root.Visible() {
		t.Error("banner visible after dismiss")
	}
}

// TestUpdateBannerNilSeam: a dev build wires no updater, so the banner never shows.
func TestUpdateBannerNilSeam(t *testing.T) {
	b := newUpdateBannerView(buildSources{})
	b.refresh()
	if b.root.Visible() {
		t.Error("banner visible with no update seam")
	}
}
