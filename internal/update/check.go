package update

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// githubRelease is the subset of the releases/latest payload the checker reads.
type githubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// checkLatest queries the latest release and reports an installable update when
// the release is strictly newer than current AND ships a binary for this platform.
// It returns ok=false (no error) when already up-to-date or when the platform has
// no asset — both are normal, not failures. A transport/parse problem is returned
// as an error for the caller to log and treat as a no-op.
func checkLatest(ctx context.Context, client *http.Client, current string) (available, bool, error) {
	rel, err := fetchLatest(ctx, client)
	if err != nil {
		return available{}, false, err
	}
	if compare(rel.TagName, current) <= 0 {
		return available{}, false, nil // not newer
	}
	asset := assetName()
	binURL, ok := assetURL(rel, asset)
	if !ok {
		return available{}, false, nil // no build for this platform in the release
	}
	sumURL, ok := assetURL(rel, checksumsAsset)
	if !ok {
		return available{}, false, fmt.Errorf("release %s has no %s", rel.TagName, checksumsAsset)
	}
	sums, err := fetchBytes(ctx, client, sumURL)
	if err != nil {
		return available{}, false, err
	}
	sum, ok := checksumFor(sums, asset)
	if !ok {
		return available{}, false, fmt.Errorf("no checksum for %s in %s", asset, checksumsAsset)
	}
	return available{version: rel.TagName, assetURL: binURL, checksum: sum}, true, nil
}

// assetURL returns the download URL of the release asset named name.
func assetURL(rel githubRelease, name string) (string, bool) {
	for _, a := range rel.Assets {
		if a.Name == name {
			return a.BrowserDownloadURL, true
		}
	}
	return "", false
}

// checksumFor finds the SHA-256 of asset in a `sha256sum`-format checksums file
// (each line "<hex>  <filename>").
func checksumFor(data []byte, asset string) (string, bool) {
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 2 && fields[1] == asset {
			return fields[0], true
		}
	}
	return "", false
}

func fetchLatest(ctx context.Context, client *http.Client) (githubRelease, error) {
	data, err := fetchBytes(ctx, client, latestReleaseURL)
	if err != nil {
		return githubRelease{}, err
	}
	var rel githubRelease
	if err := json.Unmarshal(data, &rel); err != nil {
		return githubRelease{}, fmt.Errorf("parse release: %w", err)
	}
	return rel, nil
}

// newGetRequest builds a GET carrying the User-Agent GitHub's API requires.
// Both the release check and the asset download go through it, so the header is
// set in exactly one place.
func newGetRequest(ctx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	return req, nil
}

func fetchBytes(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, err := newGetRequest(ctx, url)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}
