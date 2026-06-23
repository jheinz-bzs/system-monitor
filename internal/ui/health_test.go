package ui

import "testing"

func TestHealthThresholdsClassify(t *testing.T) {
	// CPU thresholds: warn 50, crit 75.
	cases := []struct {
		pct  float64
		want statusKind
	}{
		{0, status.Healthy},
		{49, status.Healthy},
		{50, status.Warning},
		{74, status.Warning},
		{75, status.Critical},
		{100, status.Critical},
	}
	for _, c := range cases {
		if got := cpuHealth.classify(c.pct); got != c.want {
			t.Errorf("cpuHealth.classify(%v) = %v, want %v", c.pct, got, c.want)
		}
	}
}

func TestAggregateHealth(t *testing.T) {
	H, W, C := status.Healthy, status.Warning, status.Critical
	cases := []struct {
		name   string
		states []statusKind
		want   statusKind
	}{
		{"empty reads healthy", nil, H},
		{"one elevated stays healthy", []statusKind{W, H, H}, H},
		{"two elevated -> warning", []statusKind{W, W, H}, W},
		{"one warn one crit -> warning", []statusKind{W, C, H}, W},
		{"two critical -> critical", []statusKind{C, C, H}, C},
		{"no greens -> critical", []statusKind{W, W, W}, C},
	}
	for _, c := range cases {
		if got := aggregateHealth(c.states); got != c.want {
			t.Errorf("%s: aggregateHealth(%v) = %v, want %v", c.name, c.states, got, c.want)
		}
	}
}
