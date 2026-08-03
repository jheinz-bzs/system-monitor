// Session option assembly (issue #82). The record toggle used to start a plain
// CSV session behind a bare save dialog; compact output and a top-processes
// sidecar were reachable only from the headless recording binary's flags. This
// file gives the GUI modal — and the CLI, if it ever wants it — a single pure
// translation from a session spec to the recorder's format options. It stays in
// the recorder package because that is where the options live and where the
// package's Fyne-free, monitor-free rule keeps it natively testable: the spec
// is plain data, and the two impure seams (the top-processes sample and the
// sidecar file handle) are injected rather than imported.

package recorder

import (
	"io"
	"sort"
	"strconv"
	"strings"
)

// OptionsSpec is the session shape a caller chooses at start — the GUI modal's
// toggles, or the headless binary's --compact / --processes flags. It is the
// pure input to Options, which turns it into the format options a session
// starts with. Recorder options are construction-time (ADR-012), so a spec is
// fixed for the session's lifetime.
type OptionsSpec struct {
	// Compact writes gzip-compressed CSV (.csv.gz) instead of plain CSV.
	Compact bool

	// TopN is how many busiest-by-CPU processes land in the top-processes
	// sidecar each tick; 0 leaves the sidecar off.
	TopN int
}

// snapshotsEveryTick is the top-processes sidecar cadence: one snapshot per
// poll tick, so a sidecar row exists for every metric row the two files join on.
const snapshotsEveryTick = 1

// Options translates a session spec into the recorder's format options: Compact
// adds gzip'd .csv.gz output; a TopN > 0 spec adds a top-processes sidecar
// driven by the injected snapshot and opened through open. snapshot and open are
// injected so the decision stays pure — the recorder imports neither Fyne nor
// monitor (ADR-012) — while composition roots wire the live collector's
// top-processes sample and file creation. A nil snapshot or open (the process
// collector failed to start, say) drops the sidecar rather than recording a
// header-only file or panicking on a nil sample.
func Options(spec OptionsSpec, snapshot ProcessSnapshot, open func() io.WriteCloser) []Option {
	var opts []Option
	if spec.Compact {
		opts = append(opts, Compact())
	}
	if spec.TopN <= 0 || snapshot == nil || open == nil {
		return opts
	}
	opts = append(opts, WithProcessSnapshots(snapshot, snapshotsEveryTick, open))
	return opts
}

// ParseTopN reads the top-processes count a caller typed, mirroring the
// headless binary's --processes semantics: blank or unparseable text reads as 0
// (sidecar off), a negative count clamps to 0, and a whole number passes
// through. A caller that can't tell the user what went wrong still degrades to
// the safe default rather than guessing.
func ParseTopN(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

// TopSamples caps a process sample to the n busiest by CPU, busiest first — the
// pure rule behind the sidecar's TopN, shared by the GUI and the headless
// binary so both record identical top-N semantics. n <= 0 returns nil (sidecar
// off); a sample shorter than n passes through uncapped. The input slice is
// copied, so the caller's snapshot is left untouched.
func TopSamples(ps []ProcessSample, n int) []ProcessSample {
	if n <= 0 || len(ps) == 0 {
		return nil
	}
	top := append([]ProcessSample(nil), ps...)
	sort.SliceStable(top, func(i, j int) bool { return top[i].CPU > top[j].CPU })
	if len(top) > n {
		top = top[:n]
	}
	return top
}
