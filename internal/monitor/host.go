package monitor

import (
	"context"
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v4/host"
)

// HostSummary is a static snapshot of the machine the app runs on, read once at
// startup (BZS253-74). It is plain data, not a polled metric — the About /
// System Info panel renders it directly, so there is no ring buffer or Source.
type HostSummary struct {
	Hostname        string
	OS              string        // gopsutil OS family, e.g. "windows"
	Platform        string        // product / distribution, e.g. "Microsoft Windows 11 Pro"
	PlatformVersion string        // platform version string
	KernelVersion   string        // OS kernel version, if available
	BootTime        time.Time     // zero when unknown
	Uptime          time.Duration // since boot
	Users           []string      // distinct logged-in usernames; nil if the lookup failed
}

// Overridable in tests; production reads gopsutil directly. Two seams so a test
// can inject a fake host, and drive the users call to error independently of the
// info call (the degradation path, below).
var (
	collectHostInfo  = host.InfoWithContext
	collectHostUsers = host.UsersWithContext
)

// Host reads static facts about the current machine in one gopsutil call, plus
// a second, degradable call for logged-in users.
func Host(ctx context.Context) (HostSummary, error) {
	info, err := collectHostInfo(ctx)
	if err != nil {
		return HostSummary{}, fmt.Errorf("collect host info: %w", err)
	}

	var boot time.Time
	if info.BootTime > 0 {
		boot = time.Unix(int64(info.BootTime), 0)
	}

	return HostSummary{
		Hostname:        info.Hostname,
		OS:              info.OS,
		Platform:        info.Platform,
		PlatformVersion: info.PlatformVersion,
		KernelVersion:   info.KernelVersion,
		BootTime:        boot,
		Uptime:          time.Duration(info.Uptime) * time.Second,
		Users:           loginNames(ctx),
	}, nil
}

// loginNames returns the distinct logged-in usernames. The users lookup is
// flaky on Windows, so a failure degrades to no users — the panel omits that
// row rather than the whole summary failing (BZS253-74).
func loginNames(ctx context.Context) []string {
	users, err := collectHostUsers(ctx)
	if err != nil {
		return nil
	}
	seen := make(map[string]bool, len(users))
	var names []string
	for _, u := range users {
		if u.User == "" || seen[u.User] {
			continue
		}
		seen[u.User] = true
		names = append(names, u.User)
	}
	return names
}
