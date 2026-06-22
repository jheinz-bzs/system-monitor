package ui

import "testing"

// fixedConns adapts a literal row set to allConnsSource, returning a fresh slice
// per call (Snapshot sorts the returned slice in place), like the app.go adapter.
func fixedConns(rows []connRow) allConnsSource {
	return allConnsSourceFunc(func() []connRow {
		out := make([]connRow, len(rows))
		copy(out, rows)
		return out
	})
}

func testConnRows() []connRow {
	return []connRow{
		{proto: portProtoTCP, localAddr: "192.168.1.24:52344", remoteAddr: "140.82.112.21:443", state: connStateEstablished, process: "chrome", pid: 3412},
		{proto: portProtoUDP, localAddr: "192.168.1.24:5353", remoteAddr: "", state: "", process: "", pid: 188},
		{proto: portProtoTCP, localAddr: "0.0.0.0:8080", remoteAddr: "*:*", state: connStateListen, process: "docker-proxy", pid: 991},
	}
}

// connCol indexes the columns Snapshot emits, in wireframe order.
const (
	connColProtoIdx = iota
	connColLocalIdx
	connColRemoteIdx
	connColStateIdx
	connColOwnerIdx
	connColPIDIdx
	connColLinkIdx
)

func TestConnsSnapshotCounts(t *testing.T) {
	m := newConnsTableModel(fixedConns(testConnRows()))
	m.Snapshot()

	if got := m.activeCount(); got != 3 {
		t.Errorf("activeCount = %d, want 3", got)
	}
	if got := m.establishedCount(); got != 1 {
		t.Errorf("establishedCount = %d, want 1", got)
	}
}

func TestConnsUnresolvedOwnerShowsDash(t *testing.T) {
	m := newConnsTableModel(fixedConns(testConnRows()))
	cells := m.Snapshot()

	// Find the UDP row (pid 188, empty state) and assert its owner and state both
	// render as the dash placeholder, and its jump link is empty.
	var found bool
	for _, row := range cells {
		if row[connColPIDIdx].text != "188" {
			continue
		}
		found = true
		if got := row[connColOwnerIdx].text; got != labelNoProcess {
			t.Errorf("unresolved owner = %q, want %q", got, labelNoProcess)
		}
		if got := row[connColStateIdx].text; got != labelNoProcess {
			t.Errorf("empty state = %q, want %q", got, labelNoProcess)
		}
		if got := row[connColLinkIdx].text; got != "" {
			t.Errorf("unresolved jump link = %q, want empty", got)
		}
	}
	if !found {
		t.Fatal("UDP row (pid 188) missing from snapshot")
	}
}

func TestConnsProtocolFilter(t *testing.T) {
	m := newConnsTableModel(fixedConns(testConnRows()))
	m.setProtoFilter(protoFilterUDP)
	cells := m.Snapshot()

	if len(cells) != 1 {
		t.Fatalf("UDP filter rows = %d, want 1", len(cells))
	}
	if got := cells[0][connColProtoIdx].text; got != string(portProtoUDP) {
		t.Errorf("filtered proto = %q, want %q", got, portProtoUDP)
	}
	// Counts tally the UNFILTERED list, so they still describe the whole machine.
	if got := m.activeCount(); got != 3 {
		t.Errorf("activeCount = %d, want 3 (unfiltered)", got)
	}
}

func TestConnsTextFilterMatchesAddressProcessAndPID(t *testing.T) {
	m := newConnsTableModel(fixedConns(testConnRows()))

	m.setFilter("chrome")
	if got := len(m.Snapshot()); got != 1 {
		t.Errorf("filter 'chrome' rows = %d, want 1", got)
	}

	m.setFilter("8080")
	cells := m.Snapshot()
	if len(cells) != 1 || cells[0][connColPIDIdx].text != "991" {
		t.Errorf("filter '8080' = %v, want single row pid 991", cells)
	}

	m.setFilter("140.82")
	if got := len(m.Snapshot()); got != 1 {
		t.Errorf("filter '140.82' (remote addr) rows = %d, want 1", got)
	}
}

func TestConnStatePill(t *testing.T) {
	cases := []struct {
		state connState
		want  statusKind
	}{
		{connStateEstablished, status.Healthy},
		{connStateListen, status.Healthy},
		{connStateClose, status.Critical},
		{connStateTimeWait, status.Warning},
		{"", status.Neutral},
	}
	for _, c := range cases {
		if got := c.state.pill(); got != c.want {
			t.Errorf("%q pill = %v, want %v", c.state, got, c.want)
		}
	}
}
