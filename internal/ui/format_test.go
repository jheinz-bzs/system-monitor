package ui

import (
	"testing"
	"time"
)

func TestFormatBytesShort(t *testing.T) {
	cases := map[uint64]string{
		0:                  "0B",
		512:                "512B",
		88 << 20:           "88M",
		612 << 20:          "612M",
		1932735283:         "1.8G", // ~1.8 GiB keeps one decimal under 10
		uint64(20) << 30:   "20G",
		uint64(2)<<40 + 99: "2.0T",
	}
	for in, want := range cases {
		if got := formatBytesShort(in); got != want {
			t.Errorf("formatBytesShort(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestFormatSpan(t *testing.T) {
	cases := map[time.Duration]string{
		45 * time.Second: "45 s",
		time.Minute:      "1 min",
		10 * time.Minute: "10 min",
		90 * time.Second: "2 min", // rounds to the nearest minute
	}
	for in, want := range cases {
		if got := formatSpan(in); got != want {
			t.Errorf("formatSpan(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestShortUsername(t *testing.T) {
	cases := map[string]string{
		"DESKTOP-1\\joe": "joe",
		"root":           "root",
		"":               "",
	}
	for in, want := range cases {
		if got := shortUsername(in); got != want {
			t.Errorf("shortUsername(%q) = %q, want %q", in, got, want)
		}
	}
}
