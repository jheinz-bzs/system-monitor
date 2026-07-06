package ui

// Persisted user preferences (BZS253-72). Settings survive restarts via Fyne's
// app.Preferences() — no config file or database. This file is the single home
// for the typed preference keys, their defaults, and the typed accessors the
// settings tab and the composition root read; nothing else types a raw pref key.

import (
	"slices"
	"time"
)

// prefKey namespaces the persisted preference keys so call sites read
// prefKey.StartTab rather than a loose string literal (no-string-literals). The
// string values are the on-disk keys: keep them stable across releases, or
// persisted settings silently reset to their defaults.
var prefKey = struct {
	StartTab        string
	MemoryCap       string
	PollSeconds     string
	Theme           string
	AutoUpdate      string
	LastSeenVersion string
}{
	StartTab:        "startTab",
	MemoryCap:       "memoryCapEnabled",
	PollSeconds:     "pollSeconds",
	Theme:           "theme",
	AutoUpdate:      "autoUpdate",
	LastSeenVersion: "lastSeenVersion",
}

// Preference defaults, returned whenever a key is unset — so a fresh install,
// or a key removed by a future release, always yields the app's shipped
// behavior. defaultPollSeconds is the same 1s cadence the ring buffers resolve
// at (see pollInterval / metrics.HistoryCapacity).
const (
	defaultStartTab  = tabOverview
	defaultMemoryCap = true
	// defaultPollSeconds must stay a member of pollSecondsAllowed, or
	// pollInterval() would clamp the default away as out-of-range.
	defaultPollSeconds = 1
	defaultTheme       = themeDark
	// defaultAutoUpdate is off: self-update is click-to-confirm by default
	// (BZS253-71's "no silent background replacement"); enabling this is the
	// user's explicit opt-in to auto-install on next launch (ADR-010).
	defaultAutoUpdate = false
)

// pollSecondsAllowed is the set of sampling cadences the settings UI offers, in
// seconds. A stored value outside this set falls back to the default, so a
// hand-edited preference can't wedge the poll loop with a zero or negative tick.
var pollSecondsAllowed = []int{1, 2, 5}

// themeChoice selects the active color palette. Stored as its int value, so the
// theme key needs no string vocabulary.
type themeChoice int

const (
	themeDark themeChoice = iota
	themeLight
)

// prefStore is the slice of fyne.Preferences the settings layer needs: typed
// get-with-fallback and set, per value kind. Defined at the consumer and kept
// narrow (idiomatic Go), so tests supply a map-backed fake without implementing
// fyne.Preferences' full surface; fyne.Preferences satisfies it structurally.
type prefStore interface {
	BoolWithFallback(key string, fallback bool) bool
	SetBool(key string, value bool)
	IntWithFallback(key string, fallback int) int
	SetInt(key string, value int)
	StringWithFallback(key string, fallback string) string
	SetString(key string, value string)
}

// settings reads and writes the app's persisted preferences with typed
// accessors and built-in defaults, over a prefStore (fyne.Preferences in
// production). Every getter returns the default when its key is unset or holds
// an out-of-range value, so no stored preference can select a nonexistent tab,
// stall the poll loop, or pick a palette that doesn't exist.
type settings struct {
	store prefStore
}

// newSettings wraps a preference store in the typed accessor layer.
func newSettings(store prefStore) settings { return settings{store: store} }

// startTab is the tab shown on launch. An out-of-range stored value (e.g. a
// removed tab) falls back to the default rather than selecting nothing.
func (s settings) startTab() tabID {
	id := tabID(s.store.IntWithFallback(prefKey.StartTab, int(defaultStartTab)))
	if id > tabSettings {
		return defaultStartTab
	}
	return id
}

// setStartTab persists the launch tab.
func (s settings) setStartTab(id tabID) { s.store.SetInt(prefKey.StartTab, int(id)) }

// memoryCapEnabled reports whether the GC soft memory limit is applied at
// startup (see installDefaultMemoryLimit). Defaults on.
func (s settings) memoryCapEnabled() bool {
	return s.store.BoolWithFallback(prefKey.MemoryCap, defaultMemoryCap)
}

// setMemoryCapEnabled persists the memory-cap toggle.
func (s settings) setMemoryCapEnabled(on bool) { s.store.SetBool(prefKey.MemoryCap, on) }

// pollInterval is the sampling/redraw cadence applied at startup. A stored
// value outside pollSecondsAllowed falls back to the default.
func (s settings) pollInterval() time.Duration {
	secs := s.store.IntWithFallback(prefKey.PollSeconds, defaultPollSeconds)
	if !slices.Contains(pollSecondsAllowed, secs) {
		secs = defaultPollSeconds
	}
	return time.Duration(secs) * time.Second
}

// setPollSeconds persists the poll cadence, in seconds.
func (s settings) setPollSeconds(secs int) { s.store.SetInt(prefKey.PollSeconds, secs) }

// theme is the active palette choice. An unknown stored value falls back to the
// default (dark) so the app always has a valid palette.
func (s settings) theme() themeChoice {
	t := themeChoice(s.store.IntWithFallback(prefKey.Theme, int(defaultTheme)))
	if t < themeDark || t > themeLight {
		return defaultTheme
	}
	return t
}

// setTheme persists the palette choice.
func (s settings) setTheme(t themeChoice) { s.store.SetInt(prefKey.Theme, int(t)) }

// autoUpdateEnabled reports whether a found update is installed automatically on
// next launch (vs. waiting for the user to click). Defaults off (ADR-010).
func (s settings) autoUpdateEnabled() bool {
	return s.store.BoolWithFallback(prefKey.AutoUpdate, defaultAutoUpdate)
}

// setAutoUpdateEnabled persists the auto-install opt-in.
func (s settings) setAutoUpdateEnabled(on bool) { s.store.SetBool(prefKey.AutoUpdate, on) }

// lastSeenVersion is the build version at the last launch that showed (or, on a
// fresh install, silently recorded) the "What's New" page (BZS253-78). Empty
// means unset — a brand-new install that has never recorded a version. The
// launch compare against the current build version is the whole "don't show
// again until updated" mechanism; no seen-flags or timestamps.
func (s settings) lastSeenVersion() string {
	return s.store.StringWithFallback(prefKey.LastSeenVersion, "")
}

// setLastSeenVersion records the build version whose "What's New" page has been
// dismissed (or recorded on first install), suppressing the page until the next
// version change.
func (s settings) setLastSeenVersion(v string) { s.store.SetString(prefKey.LastSeenVersion, v) }
