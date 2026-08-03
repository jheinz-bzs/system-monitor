//go:build unix

package monitor

import (
	"io/fs"
	"syscall"
)

// fileIdentity reports the identity of a regular file with more than one hard
// link, so the walk can count the inode once. ok is false for single-link
// files — the common case, which then needs no dedup — and for anything whose
// metadata can't be read. The stat came with the walk, so this never touches
// the filesystem.
func fileIdentity(_ string, info fs.FileInfo) (id fileID, ok bool) {
	if !info.Mode().IsRegular() {
		return fileID{}, false
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || st.Nlink < 2 {
		return fileID{}, false
	}
	return fileID{volume: uint64(st.Dev), index: uint64(st.Ino)}, true
}
