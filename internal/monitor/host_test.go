package monitor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shirou/gopsutil/v4/host"
)

// withFakeHost swaps the gopsutil seams for the duration of a test and restores
// them after, so cases can inject a known host and an independently-failing
// users call.
func withFakeHost(t *testing.T, info *host.InfoStat, infoErr error, users []host.UserStat, usersErr error) {
	t.Helper()
	origInfo, origUsers := collectHostInfo, collectHostUsers
	t.Cleanup(func() { collectHostInfo, collectHostUsers = origInfo, origUsers })
	collectHostInfo = func(context.Context) (*host.InfoStat, error) { return info, infoErr }
	collectHostUsers = func(context.Context) ([]host.UserStat, error) { return users, usersErr }
}

func TestHostPopulatesFields(t *testing.T) {
	boot := time.Now().Add(-90 * time.Minute).Truncate(time.Second)
	withFakeHost(t, &host.InfoStat{
		Hostname:        "devbox",
		OS:              "windows",
		Platform:        "Microsoft Windows 11 Pro",
		PlatformVersion: "10.0.26100",
		KernelVersion:   "10.0.26100.1",
		BootTime:        uint64(boot.Unix()),
		Uptime:          5400, // 90 min
	}, nil,
		[]host.UserStat{{User: "joe"}, {User: "joe"}, {User: ""}, {User: "admin"}}, nil)

	got, err := Host(context.Background())
	if err != nil {
		t.Fatalf("Host: %v", err)
	}
	if got.Hostname != "devbox" || got.KernelVersion != "10.0.26100.1" {
		t.Errorf("fields not populated: %+v", got)
	}
	if !got.BootTime.Equal(boot) {
		t.Errorf("BootTime = %v, want %v", got.BootTime, boot)
	}
	if got.Uptime != 90*time.Minute {
		t.Errorf("Uptime = %v, want 90m", got.Uptime)
	}
	// Distinct, non-empty users only, in order.
	if want := []string{"joe", "admin"}; !equalStrings(got.Users, want) {
		t.Errorf("Users = %v, want %v", got.Users, want)
	}
}

func TestHostOmitsUsersOnError(t *testing.T) {
	withFakeHost(t, &host.InfoStat{Hostname: "devbox", Uptime: 60}, nil,
		nil, errors.New("access denied"))

	got, err := Host(context.Background())
	if err != nil {
		t.Fatalf("Host must not fail when only the users call errors: %v", err)
	}
	if got.Hostname != "devbox" {
		t.Errorf("summary lost its primary fields: %+v", got)
	}
	if len(got.Users) != 0 {
		t.Errorf("Users = %v, want empty (row omitted)", got.Users)
	}
}

func TestHostFailsWhenInfoFails(t *testing.T) {
	withFakeHost(t, nil, errors.New("boom"), nil, nil)
	if _, err := Host(context.Background()); err == nil {
		t.Fatal("Host should propagate a failing info call")
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
