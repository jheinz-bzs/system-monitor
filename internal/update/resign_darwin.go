//go:build darwin

package update

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// resignAfterSwap re-applies the ad-hoc code signature to the enclosing .app
// bundle after a self-update replaced the executable. The bundle ships signed
// (see .github/workflows/release.yml) so Apple Silicon will launch it; a swap
// of Contents/MacOS/system-monitor invalidates the bundle's _CodeSignature
// seal, which would make the next launch fail with the same "damaged" block.
// Re-signing with the same ad-hoc identity (no Developer ID) restores it.
func resignAfterSwap(exe string) error {
	bundle := bundleRoot(exe)
	if bundle == "" {
		// Not running from a bundle (bare binary / dev build) — nothing to seal.
		return nil
	}
	cmd := exec.Command("codesign", "--force", "--deep", "--sign", "-", bundle)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("re-sign bundle %s: %w: %s", bundle, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// bundleRoot returns the .app directory containing exe, or "" when exe is not
// inside a standard bundle layout (…/Contents/MacOS/system-monitor).
func bundleRoot(exe string) string {
	dir := filepath.Dir(exe) // …/Contents/MacOS
	if filepath.Base(dir) != "MacOS" {
		return ""
	}
	if filepath.Base(filepath.Dir(dir)) != "Contents" {
		return ""
	}
	return filepath.Dir(filepath.Dir(dir))
}
