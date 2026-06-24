package update

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func serve(payload []byte) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write(payload)
	}))
}

func TestDownloadVerifiedSuccess(t *testing.T) {
	payload := []byte("a freshly built binary")
	sum := sha256.Sum256(payload)
	srv := serve(payload)
	defer srv.Close()

	dir := t.TempDir()
	path, err := downloadVerified(context.Background(), srv.Client(), srv.URL, hex.EncodeToString(sum[:]), dir)
	if err != nil {
		t.Fatalf("downloadVerified: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("downloaded bytes differ from payload")
	}
}

func TestDownloadVerifiedChecksumMismatchAborts(t *testing.T) {
	srv := serve([]byte("tampered payload"))
	defer srv.Close()

	dir := t.TempDir()
	_, err := downloadVerified(context.Background(), srv.Client(), srv.URL, "deadbeef", dir)
	if err == nil {
		t.Fatal("expected a checksum-mismatch error, got nil")
	}
	// The temp file must be cleaned up so a bad download can't be swapped in.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("temp file left behind after mismatch: %v", entries)
	}
}
