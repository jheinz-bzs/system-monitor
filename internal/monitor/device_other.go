//go:build !linux

package monitor

import "io/fs"

// devOf reports the filesystem device a FileInfo lives on. Non-Linux platforms
// have no per-device identity available from the stat metadata the walk already
// holds, so it reports 0 — the walk then sees every directory as the root's
// filesystem and never prunes mount points (there is no mount-point hierarchy
// below a Windows drive root to prune anyway).
func devOf(fs.FileInfo) uint64 { return 0 }
