//go:build linux

package monitor

import (
	"io/fs"
	"syscall"
)

// devOf reports the filesystem device a FileInfo lives on. The walk uses it to
// recognize mount points below a volume root: a directory whose device differs
// from the root's is another filesystem (e.g. /proc or /home under /) and must
// not be counted against this volume. st.Dev is only meaningful on Linux, so
// the non-Linux build of devOf always reports 0 and the walk never prunes.
func devOf(info fs.FileInfo) uint64 {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return 0
	}
	return st.Dev
}
