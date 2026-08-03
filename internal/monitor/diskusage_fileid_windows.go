//go:build windows

package monitor

import (
	"io/fs"
	"os"
	"syscall"
)

// fileIdentity reports the identity of a regular file with more than one hard
// link. Windows stat metadata carries no link count or file ID, so the check
// opens the file and asks the kernel via GetFileInformationByHandle — a
// syscall paid for every file, since the link count is unknowable without a
// handle. ok is false for single-link files and when the lookup fails (the
// file is then counted normally).
func fileIdentity(path string, info fs.FileInfo) (id fileID, ok bool) {
	if !info.Mode().IsRegular() {
		return fileID{}, false
	}
	f, err := os.Open(path)
	if err != nil {
		return fileID{}, false
	}
	defer f.Close()
	var bi syscall.ByHandleFileInformation
	if err := syscall.GetFileInformationByHandle(syscall.Handle(f.Fd()), &bi); err != nil {
		return fileID{}, false
	}
	if bi.NumberOfLinks < 2 {
		return fileID{}, false
	}
	return fileID{
		volume: uint64(bi.VolumeSerialNumber),
		index:  uint64(bi.FileIndexHigh)<<32 | uint64(bi.FileIndexLow),
	}, true
}
