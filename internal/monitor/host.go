package monitor

import (
	"context"

	"github.com/shirou/gopsutil/v4/host"
)

// HostSummary is a minimal snapshot of the machine the app is running on.
type HostSummary struct {
	Hostname string
	OS       string
	Platform string
}

// Host returns basic information about the current host. It is a thin wrapper
// over gopsutil and mainly serves to verify the dependency is wired up; the
// real collectors are added in later stories.
func Host(ctx context.Context) (HostSummary, error) {
	info, err := host.InfoWithContext(ctx)
	if err != nil {
		return HostSummary{}, err
	}
	return HostSummary{
		Hostname: info.Hostname,
		OS:       info.OS,
		Platform: info.Platform,
	}, nil
}
