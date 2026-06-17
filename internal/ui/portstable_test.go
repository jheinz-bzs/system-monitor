package ui

import "testing"

// fixedPorts adapts a literal row set to allPortsSource, returning a fresh slice
// per call (Snapshot sorts the returned slice in place), like the app.go adapter.
func fixedPorts(rows []portRow) allPortsSource {
	return allPortsSourceFunc(func() []portRow {
		out := make([]portRow, len(rows))
		copy(out, rows)
		return out
	})
}

func testPortRows() []portRow {
	return []portRow{
		{proto: portProtoTCP, port: 443, process: "nginx", pid: 771, localAddr: "0.0.0.0:443"},
		{proto: portProtoUDP, port: 53, process: "", pid: 212, localAddr: "127.0.0.53:53"},
		{proto: portProtoTCP, port: 22, process: "sshd", pid: 604, localAddr: "0.0.0.0:22"},
	}
}

// portCol indexes the columns Snapshot emits, in wireframe order.
const (
	portColProtoIdx = iota
	portColPortIdx
	portColStateIdx
	portColOwnerIdx
	portColPIDIdx
	portColAddrIdx
	portColLinkIdx
)

func TestPortsSnapshotSortsByPortAscending(t *testing.T) {
	m := newPortsTableModel(fixedPorts(testPortRows()))
	cells := m.Snapshot()

	want := []string{"22", "53", "443"}
	for i, w := range want {
		if got := cells[i][portColPortIdx].text; got != w {
			t.Errorf("row %d port = %q, want %q (ascending)", i, got, w)
		}
	}
}

func TestPortsUnresolvedOwnerShowsDash(t *testing.T) {
	m := newPortsTableModel(fixedPorts(testPortRows()))
	cells := m.Snapshot()

	// Ascending sort: row 0 = port 22 (sshd), row 1 = port 53 (unresolved → dash),
	// row 2 = port 443 (nginx).
	if got := cells[1][portColOwnerIdx].text; got != labelNoProcess {
		t.Errorf("unresolved owner = %q, want %q", got, labelNoProcess)
	}
	if got := cells[2][portColOwnerIdx].text; got != "nginx" {
		t.Errorf("resolved owner = %q, want %q", got, "nginx")
	}

	// The trailing cross-nav link carries the resolved name + arrow, and is empty
	// when there's no owner to jump to.
	if got := cells[2][portColLinkIdx].text; got != "nginx"+labelJumpSuffix {
		t.Errorf("jump link = %q, want %q", got, "nginx"+labelJumpSuffix)
	}
	if got := cells[1][portColLinkIdx].text; got != "" {
		t.Errorf("unresolved jump link = %q, want empty", got)
	}
}

func TestPortsProtocolFilter(t *testing.T) {
	m := newPortsTableModel(fixedPorts(testPortRows()))
	m.setProtoFilter(protoFilterUDP)
	cells := m.Snapshot()

	if len(cells) != 1 {
		t.Fatalf("UDP filter rows = %d, want 1", len(cells))
	}
	if got := cells[0][portColProtoIdx].text; got != string(portProtoUDP) {
		t.Errorf("filtered proto = %q, want %q", got, portProtoUDP)
	}
	// listeningCount tallies the UNFILTERED list, so it still reports all ports.
	if got := m.listeningCount(); got != 3 {
		t.Errorf("listeningCount = %d, want 3 (unfiltered)", got)
	}
}

func TestPortsTextFilterMatchesPortAndProcess(t *testing.T) {
	m := newPortsTableModel(fixedPorts(testPortRows()))

	m.setFilter("nginx")
	if got := len(m.Snapshot()); got != 1 {
		t.Errorf("filter 'nginx' rows = %d, want 1", got)
	}

	m.setFilter("22")
	cells := m.Snapshot()
	if len(cells) != 1 || cells[0][portColPIDIdx].text != "604" {
		t.Errorf("filter '22' = %v, want single row pid 604", cells)
	}
}
