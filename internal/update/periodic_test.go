package update

import (
	"context"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

// TestShouldCheck locks the skip-while-available gate: a periodic check is worth
// running only when the controller sits at StatusIdle. Every other state means
// the previous outcome still stands (banner up, work in flight, or a failed
// install that must not nag into a retry loop).
func TestShouldCheck(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status Status
		want   bool
	}{
		{name: "idle is checkable", status: StatusIdle, want: true},
		{name: "in-flight check is not", status: StatusChecking, want: false},
		{name: "available release is not re-queried", status: StatusAvailable, want: false},
		{name: "download in progress is not", status: StatusDownloading, want: false},
		{name: "install in progress is not", status: StatusInstalling, want: false},
		{name: "failed install is not retried", status: StatusFailed, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldCheck(tc.status); got != tc.want {
				t.Errorf("shouldCheck(%v) = %v, want %v", tc.status, got, tc.want)
			}
		})
	}
}

// controllerWithClient builds a controller whose HTTP client is the mock
// transport, so Check runs against canned responses without a network.
func controllerWithClient(current string, client *http.Client) *Controller {
	c := NewController(current, ModeSelf, nil)
	c.client = client
	return c
}

// upToDateClient serves the same version the controller compares against, so a
// Check reports "no update" and returns to StatusIdle.
func upToDateClient() *http.Client {
	return &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return newResp(http.StatusOK, `{"tag_name":"v1.0.0","assets":[]}`), nil
	})}
}

// newerReleaseClient serves a strictly-newer release for this platform plus its
// checksums asset, so Check lands on StatusAvailable. calls counts only hits to
// the rate-limited releases API, not the asset download.
func newerReleaseClient() (*http.Client, *int32) {
	var calls int32
	const (
		newTag  = "v9.9.9"
		sumURL  = "https://example.test/checksums"
		wantSum = "0123abc"
	)
	asset := assetName()
	relJSON := fmt.Sprintf(
		`{"tag_name":%q,"assets":[{"name":%q,"browser_download_url":%q},{"name":%q,"browser_download_url":%q}]}`,
		newTag, asset, "https://example.test/bin", checksumsAsset, sumURL)
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.URL.String() {
		case latestReleaseURL:
			atomic.AddInt32(&calls, 1)
			return newResp(http.StatusOK, relJSON), nil
		case sumURL:
			return newResp(http.StatusOK, wantSum+"  "+asset+"\n"), nil
		default:
			return newResp(http.StatusNotFound, ""), nil
		}
	})}
	return client, &calls
}

// TestRunPeriodicChecksChecksImmediately: the loop must query on launch, before
// the first tick. A zero interval exercises the DefaultCheckInterval fallback,
// so only the launch-time check can fire within the test.
func TestRunPeriodicChecksChecksImmediately(t *testing.T) {
	var calls int32
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		atomic.AddInt32(&calls, 1)
		return newResp(http.StatusOK, `{"tag_name":"v1.0.0","assets":[]}`), nil
	})}
	c := controllerWithClient("v1.0.0", client)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	RunPeriodicChecks(ctx, c, 0, nil)

	deadline := time.Now().Add(time.Second)
	for atomic.LoadInt32(&calls) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("RunPeriodicChecks never ran the launch-time check")
		}
		time.Sleep(time.Millisecond)
	}
}

// TestRunPeriodicChecksSkipsWhileAvailable: once the launch-time check surfaces
// a release, the banner is up and the loop must stop hitting the API — a second
// query would only burn rate-limit budget. With a 10ms interval the loop ticks
// many times during the observation window; all must be skipped.
func TestRunPeriodicChecksSkipsWhileAvailable(t *testing.T) {
	client, calls := newerReleaseClient()
	c := controllerWithClient("v1.0.0", client)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	RunPeriodicChecks(ctx, c, 10*time.Millisecond, nil)

	deadline := time.Now().Add(time.Second)
	for c.Snapshot().State != StatusAvailable {
		if time.Now().After(deadline) {
			t.Fatalf("launch check never surfaced the release; state=%v", c.Snapshot().State)
		}
		time.Sleep(time.Millisecond)
	}
	time.Sleep(200 * time.Millisecond)
	if got := atomic.LoadInt32(calls); got != 1 {
		t.Fatalf("periodic loop made %d release-API calls after the banner was up, want 1", got)
	}
}

// TestRunPeriodicChecksStopsOnCancel: cancelling the session context must stop
// the loop, so no timer or in-flight query outlives shutdown. The transport
// blocks its first (and only) check until the context dies, proving the loop
// can neither start a second check while one is in flight nor re-check after
// cancellation.
func TestRunPeriodicChecksStopsOnCancel(t *testing.T) {
	var started int32
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		atomic.AddInt32(&started, 1)
		<-r.Context().Done()
		return nil, r.Context().Err()
	})}
	c := controllerWithClient("v1.0.0", client)
	ctx, cancel := context.WithCancel(context.Background())

	RunPeriodicChecks(ctx, c, time.Millisecond, nil)

	deadline := time.Now().Add(time.Second)
	for atomic.LoadInt32(&started) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("launch-time check never started")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()

	// The in-flight check unblocks on cancellation and the loop must exit; give
	// it far longer than the 1ms interval to attempt a follow-up check.
	time.Sleep(100 * time.Millisecond)
	if got := atomic.LoadInt32(&started); got != 1 {
		t.Fatalf("periodic loop started %d checks after cancel, want only the launch check", got)
	}
}

// TestRunPeriodicChecksRunsOnAvailable: the onAvailable hook fires exactly when
// a check lands on StatusAvailable — the composition root's hook for the
// auto-install preference.
func TestRunPeriodicChecksRunsOnAvailable(t *testing.T) {
	client, _ := newerReleaseClient()
	c := controllerWithClient("v1.0.0", client)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fired := make(chan struct{}, 1)
	RunPeriodicChecks(ctx, c, time.Hour, func() { fired <- struct{}{} })

	select {
	case <-fired:
	case <-time.After(time.Second):
		t.Fatal("onAvailable never ran after an available release was found")
	}
	if c.Snapshot().State != StatusAvailable {
		t.Fatalf("state after onAvailable = %v, want %v", c.Snapshot().State, StatusAvailable)
	}
}

// TestRunPeriodicChecksNoOnAvailableWhenUpToDate: the auto-install hook must
// stay quiet for an up-to-date build, not just fire-and-no-op.
func TestRunPeriodicChecksNoOnAvailableWhenUpToDate(t *testing.T) {
	var calls int32
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		atomic.AddInt32(&calls, 1)
		return newResp(http.StatusOK, `{"tag_name":"v1.0.0","assets":[]}`), nil
	})}
	c := controllerWithClient("v1.0.0", client)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fired := make(chan struct{})
	RunPeriodicChecks(ctx, c, time.Hour, func() { close(fired) })

	deadline := time.Now().Add(time.Second)
	for atomic.LoadInt32(&calls) == 0 {
		if time.Now().After(deadline) {
			t.Fatal("launch-time check never ran")
		}
		time.Sleep(time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)
	select {
	case <-fired:
		t.Fatal("onAvailable fired for an up-to-date build")
	default:
	}
}
