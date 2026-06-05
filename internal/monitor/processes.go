package monitor

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"
	"syscall"

	gnet "github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
)

// ProcessInfo is a snapshot of one running process. PID is always populated;
// fields that are permission-restricted on the running system are left at their
// zero/empty value rather than dropping the record.
type ProcessInfo struct {
	PID         int32
	Name        string
	CPUPercent  float64 // 0..100
	MemoryBytes uint64  // resident set size
	Username    string
}

// PortInfo is a snapshot of one listening port. PID is the owning process,
// matching the PID exposed in ProcessInfo so cross-tab navigation can resolve
// a port to its process.
type PortInfo struct {
	Port      uint32
	Protocol  string // "tcp" | "udp"
	LocalAddr string // "ip:port"
	PID       int32
}

// ConnectionInfo is a snapshot of one active TCP/UDP connection. State is the
// gopsutil status string (e.g. "ESTABLISHED", "LISTEN") and is empty for UDP.
type ConnectionInfo struct {
	Protocol   string // "tcp" | "udp"
	LocalAddr  string // "ip:port"
	RemoteAddr string // "ip:port", empty when unbound
	State      string
	PID        int32
}

// processSampler returns a snapshot of all running processes. It is the seam the
// collector samples through so tests can supply readings without a real OS.
type processSampler func(ctx context.Context) ([]ProcessInfo, error)

// connSampler returns a snapshot of all TCP/UDP connections. Ports are derived
// from this same data, so a single enumeration feeds both the Ports and
// Connections tabs. It is the seam the collector samples through.
type connSampler func(ctx context.Context) ([]ConnectionInfo, error)

// processOption configures a ProcessCollector at construction. It exists so
// tests can inject samplers without a separate constructor; production code uses
// the defaults.
type processOption func(*ProcessCollector)

// withProcessSampler overrides the process sampler. Tests use it to supply
// readings without a real OS.
func withProcessSampler(s processSampler) processOption {
	return func(c *ProcessCollector) { c.sampleProcs = s }
}

// withConnSampler overrides the connection sampler. Tests use it to supply
// readings without a real OS.
func withConnSampler(s connSampler) processOption {
	return func(c *ProcessCollector) { c.sampleConns = s }
}

// defaultProcessSampler enumerates processes via gopsutil. Failure to enumerate
// the process table is returned as an error; per-process field reads that fail
// (typically permission-restricted) fall back to the zero/empty value so the
// process is still included in the snapshot.
func defaultProcessSampler(ctx context.Context) ([]ProcessInfo, error) {
	procs, err := process.ProcessesWithContext(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing processes: %w", err)
	}

	out := make([]ProcessInfo, 0, len(procs))
	for _, p := range procs {
		info := ProcessInfo{PID: p.Pid}
		if name, err := p.NameWithContext(ctx); err == nil {
			info.Name = name
		}
		if cpu, err := p.CPUPercentWithContext(ctx); err == nil {
			info.CPUPercent = cpu
		}
		if mem, err := p.MemoryInfoWithContext(ctx); err == nil && mem != nil {
			info.MemoryBytes = mem.RSS
		}
		if user, err := p.UsernameWithContext(ctx); err == nil {
			info.Username = user
		}
		out = append(out, info)
	}
	return out, nil
}

// defaultConnSampler enumerates all TCP/UDP connections via gopsutil. Failure to
// enumerate is returned as an error rather than panicking.
func defaultConnSampler(ctx context.Context) ([]ConnectionInfo, error) {
	conns, err := gnet.ConnectionsWithContext(ctx, "all")
	if err != nil {
		return nil, fmt.Errorf("listing connections: %w", err)
	}

	out := make([]ConnectionInfo, 0, len(conns))
	for _, c := range conns {
		out = append(out, ConnectionInfo{
			Protocol:   protocolName(c.Type),
			LocalAddr:  formatAddr(c.Laddr),
			RemoteAddr: formatAddr(c.Raddr),
			State:      c.Status,
			PID:        c.Pid,
		})
	}
	return out, nil
}

// protocolName maps a socket type to a protocol label. Anything other than a
// stream or datagram socket reports an empty protocol.
func protocolName(sockType uint32) string {
	switch sockType {
	case syscall.SOCK_STREAM:
		return "tcp"
	case syscall.SOCK_DGRAM:
		return "udp"
	default:
		return ""
	}
}

// formatAddr renders an address as "ip:port", or "" when the address is unset
// (an unbound remote end reports port 0).
func formatAddr(a gnet.Addr) string {
	if a.IP == "" && a.Port == 0 {
		return ""
	}
	return net.JoinHostPort(a.IP, strconv.FormatUint(uint64(a.Port), 10))
}

// deriveListeningPorts extracts the listening ports from a connection snapshot:
// TCP sockets in the LISTEN state, plus UDP sockets with no remote peer (which
// have no connection state but are effectively listening).
func deriveListeningPorts(conns []ConnectionInfo) []PortInfo {
	ports := make([]PortInfo, 0)
	for _, c := range conns {
		listening := (c.Protocol == "tcp" && c.State == "LISTEN") ||
			(c.Protocol == "udp" && c.RemoteAddr == "")
		if !listening {
			continue
		}
		ports = append(ports, PortInfo{
			Port:      localPort(c.LocalAddr),
			Protocol:  c.Protocol,
			LocalAddr: c.LocalAddr,
			PID:       c.PID,
		})
	}
	return ports
}

// localPort parses the port out of an "ip:port" address, returning 0 when it
// cannot be parsed.
func localPort(addr string) uint32 {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return 0
	}
	n, err := strconv.ParseUint(port, 10, 32)
	if err != nil {
		return 0
	}
	return uint32(n)
}

// ProcessCollector samples processes, listening ports, and active connections on
// each Collect. All three are snapshot data — the latest snapshot wholly
// replaces the previous one (no history is kept) — so each is held behind a
// RWMutex and accessors return independent copies. Ports are derived from the
// connection snapshot, so one connection enumeration feeds both.
type ProcessCollector struct {
	sampleProcs processSampler
	sampleConns connSampler

	mu          sync.RWMutex
	processes   []ProcessInfo
	ports       []PortInfo
	connections []ConnectionInfo
}

// NewProcessCollector builds a collector backed by gopsutil. It takes one
// initial snapshot so the accessors return live data immediately.
func NewProcessCollector(ctx context.Context) (*ProcessCollector, error) {
	return newProcessCollector(ctx, defaultProcessSampler, defaultConnSampler)
}

// newProcessCollector is the testable constructor: it samples through the given
// seams so tests can supply readings without a real OS.
func newProcessCollector(ctx context.Context, sampleProcs processSampler, sampleConns connSampler) (*ProcessCollector, error) {
	c := &ProcessCollector{
		sampleProcs: sampleProcs,
		sampleConns: sampleConns,
	}
	if err := c.Collect(ctx); err != nil {
		return nil, err
	}
	return c, nil
}

// Collect takes a fresh snapshot of processes and connections, derives the
// listening ports, and replaces all three snapshots. It returns an error
// (rather than panicking) when either sampler fails.
func (c *ProcessCollector) Collect(ctx context.Context) error {
	procs, err := c.sampleProcs(ctx)
	if err != nil {
		return fmt.Errorf("sampling processes: %w", err)
	}
	conns, err := c.sampleConns(ctx)
	if err != nil {
		return fmt.Errorf("sampling connections: %w", err)
	}
	ports := deriveListeningPorts(conns)

	c.mu.Lock()
	c.processes = procs
	c.connections = conns
	c.ports = ports
	c.mu.Unlock()
	return nil
}

// Processes returns a copy of the latest process snapshot.
func (c *ProcessCollector) Processes() []ProcessInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]ProcessInfo, len(c.processes))
	copy(out, c.processes)
	return out
}

// Ports returns a copy of the latest listening-port snapshot.
func (c *ProcessCollector) Ports() []PortInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]PortInfo, len(c.ports))
	copy(out, c.ports)
	return out
}

// Connections returns a copy of the latest connection snapshot.
func (c *ProcessCollector) Connections() []ConnectionInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]ConnectionInfo, len(c.connections))
	copy(out, c.connections)
	return out
}
