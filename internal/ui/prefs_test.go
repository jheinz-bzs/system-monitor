package ui

import (
	"testing"
	"time"
)

// fakePrefs is a map-backed prefStore for round-trip tests — no Fyne app or
// disk. Unset keys return the caller's fallback, exactly like fyne.Preferences.
type fakePrefs struct {
	bools map[string]bool
	ints  map[string]int
}

func newFakePrefs() *fakePrefs {
	return &fakePrefs{bools: map[string]bool{}, ints: map[string]int{}}
}

func (f *fakePrefs) BoolWithFallback(key string, fallback bool) bool {
	if v, ok := f.bools[key]; ok {
		return v
	}
	return fallback
}
func (f *fakePrefs) SetBool(key string, value bool) { f.bools[key] = value }
func (f *fakePrefs) IntWithFallback(key string, fallback int) int {
	if v, ok := f.ints[key]; ok {
		return v
	}
	return fallback
}
func (f *fakePrefs) SetInt(key string, value int) { f.ints[key] = value }

// TestSettingsDefaults: unset keys yield the shipped defaults.
func TestSettingsDefaults(t *testing.T) {
	s := newSettings(newFakePrefs())
	if got := s.startTab(); got != defaultStartTab {
		t.Errorf("startTab default = %d, want %d", got, defaultStartTab)
	}
	if got := s.memoryCapEnabled(); got != defaultMemoryCap {
		t.Errorf("memoryCapEnabled default = %v, want %v", got, defaultMemoryCap)
	}
	if got := s.pollInterval(); got != defaultPollSeconds*time.Second {
		t.Errorf("pollInterval default = %v, want %v", got, defaultPollSeconds*time.Second)
	}
	if got := s.theme(); got != defaultTheme {
		t.Errorf("theme default = %d, want %d", got, defaultTheme)
	}
}

// TestSettingsRoundTrip: a set value reads back through the typed accessor.
func TestSettingsRoundTrip(t *testing.T) {
	s := newSettings(newFakePrefs())

	s.setStartTab(tabProcesses)
	if got := s.startTab(); got != tabProcesses {
		t.Errorf("startTab = %d, want %d", got, tabProcesses)
	}
	s.setMemoryCapEnabled(false)
	if s.memoryCapEnabled() {
		t.Error("memoryCapEnabled = true after setting false")
	}
	s.setPollSeconds(5)
	if got := s.pollInterval(); got != 5*time.Second {
		t.Errorf("pollInterval = %v, want %v", got, 5*time.Second)
	}
	s.setTheme(themeLight)
	if got := s.theme(); got != themeLight {
		t.Errorf("theme = %d, want %d", got, themeLight)
	}
}

// TestSettingsOutOfRangeFallsBack: stored values outside the valid set degrade
// to defaults rather than wedging the app (no nonexistent tab, no zero tick).
func TestSettingsOutOfRangeFallsBack(t *testing.T) {
	f := newFakePrefs()
	s := newSettings(f)

	f.ints[prefKey.StartTab] = int(tabSettings) + 99
	if got := s.startTab(); got != defaultStartTab {
		t.Errorf("out-of-range startTab = %d, want default %d", got, defaultStartTab)
	}
	f.ints[prefKey.PollSeconds] = 0
	if got := s.pollInterval(); got != defaultPollSeconds*time.Second {
		t.Errorf("zero pollSeconds = %v, want default %v", got, defaultPollSeconds*time.Second)
	}
	f.ints[prefKey.Theme] = 42
	if got := s.theme(); got != defaultTheme {
		t.Errorf("unknown theme = %d, want default %d", got, defaultTheme)
	}
}
