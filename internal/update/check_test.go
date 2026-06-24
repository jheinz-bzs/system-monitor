package update

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
)

// roundTripFunc adapts a func to http.RoundTripper so tests can serve canned
// release/checksum responses without a network (the card's mock RoundTripper).
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func newResp(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func TestCheckLatestAvailable(t *testing.T) {
	const (
		newTag  = "v9.9.9"
		binURL  = "https://example.test/bin"
		sumURL  = "https://example.test/checksums"
		wantSum = "0123abc"
	)
	asset := assetName() // built from this platform so the fixture matches it
	relJSON := fmt.Sprintf(
		`{"tag_name":%q,"assets":[{"name":%q,"browser_download_url":%q},{"name":%q,"browser_download_url":%q}]}`,
		newTag, asset, binURL, checksumsAsset, sumURL)
	sums := wantSum + "  " + asset + "\n" + "ffff  some-other-asset\n"

	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case latestReleaseURL:
			return newResp(http.StatusOK, relJSON), nil
		case sumURL:
			return newResp(http.StatusOK, sums), nil
		default:
			return newResp(http.StatusNotFound, ""), nil
		}
	})}

	avail, ok, err := checkLatest(context.Background(), client, "v1.0.0")
	if err != nil || !ok {
		t.Fatalf("checkLatest: ok=%v err=%v", ok, err)
	}
	if avail.version != newTag || avail.assetURL != binURL || avail.checksum != wantSum {
		t.Fatalf("checkLatest = %+v, want version=%s url=%s sum=%s", avail, newTag, binURL, wantSum)
	}
}

func TestCheckLatestUpToDate(t *testing.T) {
	relJSON := `{"tag_name":"v1.0.0","assets":[]}`
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return newResp(http.StatusOK, relJSON), nil
	})}
	// Same version, and an older one, must both report "no update", no error.
	for _, current := range []string{"v1.0.0", "v1.1.0"} {
		_, ok, err := checkLatest(context.Background(), client, current)
		if ok || err != nil {
			t.Errorf("current %s: ok=%v err=%v, want false/nil", current, ok, err)
		}
	}
}

func TestCheckLatestOfflineErrors(t *testing.T) {
	wantErr := errors.New("no network")
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, wantErr
	})}
	if _, ok, err := checkLatest(context.Background(), client, "v1.0.0"); ok || err == nil {
		t.Fatalf("checkLatest offline: ok=%v err=%v, want false/non-nil", ok, err)
	}
}
