package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// downloadVerified streams the asset at url into a temp file in dir, hashing as
// it writes, and returns the temp path only if the SHA-256 matches wantHex. dir
// should be the install directory so the later swap is a same-volume rename, not
// a cross-device copy. The temp file is removed on any error — including a
// checksum mismatch, which aborts before the binary can be swapped in.
func downloadVerified(ctx context.Context, client *http.Client, url, wantHex, dir string) (string, error) {
	req, err := newGetRequest(ctx, url)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download %s: %s", url, resp.Status)
	}

	tmp, err := os.CreateTemp(dir, assetPrefix+"update-*")
	if err != nil {
		return "", err
	}
	hash := sha256.New()
	if _, err := io.Copy(tmp, io.TeeReader(resp.Body, hash)); err != nil {
		cleanup(tmp)
		return "", fmt.Errorf("write update: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}

	got := hex.EncodeToString(hash.Sum(nil))
	if !strings.EqualFold(got, wantHex) {
		os.Remove(tmp.Name())
		return "", fmt.Errorf("checksum mismatch: got %s, want %s", got, wantHex)
	}
	return tmp.Name(), nil
}

// cleanup closes and removes a partially-written temp file.
func cleanup(f *os.File) {
	f.Close()
	os.Remove(f.Name())
}
