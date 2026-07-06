package ui

import "testing"

// TestWhatsNewDecision covers the four launch cases from BZS253-78: a version
// change with a changelog shows the page; an unchanged version doesn't; a fresh
// install records the version without showing; an update with no changelog skips.
func TestWhatsNewDecision(t *testing.T) {
	cases := []struct {
		name         string
		current      string
		lastSeen     string
		hasChangelog bool
		wantShow     bool
		wantRecord   bool
	}{
		{"updated with changelog", "v1.3.0", "v1.2.0", true, true, false},
		{"unchanged", "v1.3.0", "v1.3.0", true, false, false},
		{"fresh install", "v1.3.0", "", true, false, true},
		{"updated without changelog", "v1.3.0", "v1.2.0", false, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			show, record := whatsNewDecision(c.current, c.lastSeen, c.hasChangelog)
			if show != c.wantShow || record != c.wantRecord {
				t.Errorf("whatsNewDecision(%q,%q,%v) = (show=%v,record=%v), want (show=%v,record=%v)",
					c.current, c.lastSeen, c.hasChangelog, show, record, c.wantShow, c.wantRecord)
			}
		})
	}
}
