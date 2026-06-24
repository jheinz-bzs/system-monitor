package update

import (
	"strconv"
	"strings"
)

// semverFields is the count of dotted numeric components compared: MAJOR.MINOR.PATCH.
const semverFields = 3

// compare orders two version tags as MAJOR.MINOR.PATCH, returning -1 if a<b,
// 0 if equal, +1 if a>b. A leading "v" is optional and any pre-release/build
// suffix (-rc1, +meta) is ignored. A tag that doesn't parse sorts lowest, so a
// malformed remote tag is never treated as newer than the current version.
func compare(a, b string) int {
	pa, oka := parseSemver(a)
	pb, okb := parseSemver(b)
	switch {
	case !oka && !okb:
		return 0
	case !oka:
		return -1
	case !okb:
		return 1
	}
	for i := range semverFields {
		switch {
		case pa[i] < pb[i]:
			return -1
		case pa[i] > pb[i]:
			return 1
		}
	}
	return 0
}

// isReleaseVersion reports whether v parses as a release semver — i.e. something
// the update check can meaningfully compare against. The "dev" build version (and
// anything malformed) returns false, which disables self-update.
func isReleaseVersion(v string) bool {
	_, ok := parseSemver(v)
	return ok
}

// parseSemver extracts the three numeric components of a version tag, tolerating
// an optional "v" prefix, fewer than three components (missing ones are 0), and a
// pre-release/build suffix (dropped). It returns ok=false for anything else.
func parseSemver(s string) ([semverFields]int, bool) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "v")
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		s = s[:i] // drop pre-release / build metadata
	}
	parts := strings.Split(s, ".")
	if s == "" || len(parts) > semverFields {
		return [semverFields]int{}, false
	}
	var out [semverFields]int
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return [semverFields]int{}, false
		}
		out[i] = n
	}
	return out, true
}
