package monitor

import (
	"context"
	"fmt"
	"net"
	"runtime"
	"strconv"
	"sync"
	"syscall"

	"github.com/shirou/gopsutil/v4/cpu"
	gnet "github.com/shirou/gopsutil/v4/net"
	"github.com/shirou/gopsutil/v4/process"
)

// ProcState is the coarse process-state vocabulary the Processes tab shows
// (the wireframe's status pills and "status:" filter share it). The richer
// OS-level states (idle, wait, zombie, …) fold into these three; empty means
// the state could not be determined.
type ProcState string

const (
	StateRunning  ProcState = "running"
	StateSleeping ProcState = "sleeping"
	StateStopped  ProcState = "stopped"
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
	State       ProcState
}

// Protocol identifies the transport-layer protocol for a connection or port.
type Protocol string

const (
	protoTCP Protocol = "tcp"
	protoUDP Protocol = "udp"
)

// ConnState is the gopsutil connection state string (e.g. "ESTABLISHED", "LISTEN").
type ConnState string

const connStateListen ConnState = "LISTEN"

// PortInfo is a snapshot of one listening port. PID is the owning process,
// matching the PID exposed in ProcessInfo so cross-tab navigation can resolve
// a port to its process.
type PortInfo struct {
	Port      uint32
	Protocol  Protocol
	LocalAddr string // "ip:port"
	PID       int32
}

// ConnectionInfo is a snapshot of one active TCP/UDP connection. State is the
// gopsutil status string (e.g. "ESTABLISHED", "LISTEN") and is empty for UDP.
type ConnectionInfo struct {
	Protocol   Protocol
	LocalAddr  string // "ip:port"
	RemoteAddr string // "ip:port", empty when unbound
	State      ConnState
	PID        int32
}

// processSampler returns a snapshot of all running processes. It is the seam the
// collector samples through so tests can supply readings without a real OS.
type processSampler func(ctx context.Context) ([]ProcessInfo, error)

// connSampler returns a snapshot of all TCP/UDP connections. Ports are derived
// from this same data, so a single enumeration feeds both the Ports and
// Connections tabs. It is the seam the collector samples through.
type connSampler func(ctx context.Context) ([]ConnectionInfo, error)

// processTerminator ends one process by PID. It is the seam Terminate calls
// through so tests can observe termination requests without ending a real
// process.
type processTerminator func(ctx context.Context, pid int32) error

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

// withProcessTerminator overrides the terminator. Tests use it to observe
// termination requests without ending a real process.
func withProcessTerminator(t processTerminator) processOption {
	return func(c *ProcessCollector) { c.terminate = t }
}

// defaultProcessTerminator asks the OS to end the process gracefully (SIGTERM
// rather than SIGKILL, so the process can clean up). NewProcessWithContext
// validates the PID still exists, so a stale request resolves to a clear error
// instead of signalling a recycled PID.
func defaultProcessTerminator(ctx context.Context, pid int32) error {
	p, err := process.NewProcessWithContext(ctx, pid)
	if err != nil {
		return fmt.Errorf("finding process %d: %w", pid, err)
	}
	if err := p.TerminateWithContext(ctx); err != nil {
		return fmt.Errorf("terminating process %d: %w", pid, err)
	}
	return nil
}

// newDefaultProcessSampler builds the gopsutil-backed sampler. It is stateful:
// per-PID process handles persist between calls because gopsutil computes
// CPUPercent as the delta since that handle's previous reading — a fresh handle
// every poll would instead average over the process's entire lifetime, which
// overstates busy processes and never tracks live changes. Handles for exited
// processes are dropped each call so the map cannot grow unboundedly.
func newDefaultProcessSampler() processSampler {
	handles := make(map[int32]*process.Process)
	// gopsutil reports per-process CPU relative to ONE core (a process
	// saturating four cores reads 400%), while the CPU charts plot a
	// machine-wide 0..100. Dividing by the logical core count puts process
	// rows on the same scale as the charts. Resolved lazily so the first call
	// supplies the context.
	var cores float64

	return func(ctx context.Context) ([]ProcessInfo, error) {
		if cores == 0 {
			cores = float64(logicalCoreCount(ctx))
		}
		procs, err := process.ProcessesWithContext(ctx)
		if err != nil {
			return nil, fmt.Errorf("listing processes: %w", err)
		}

		alive := make(map[int32]struct{}, len(procs))
		out := make([]ProcessInfo, 0, len(procs))
		for _, p := range procs {
			alive[p.Pid] = struct{}{}
			h, ok := handles[p.Pid]
			if !ok {
				h = p
				handles[p.Pid] = h
			}
			out = append(out, readProcess(ctx, h, cores))
		}
		for pid := range handles {
			if _, ok := alive[pid]; !ok {
				delete(handles, pid)
			}
		}
		return out, nil
	}
}

// logicalCoreCount returns the machine's logical core count from the same
// gopsutil authority CPUInfo reports, so the table's normalized percentages
// stay consistent with the advertised core count. It falls back to
// runtime.NumCPU (which can differ under CPU affinity limits) only when
// gopsutil cannot read the count.
func logicalCoreCount(ctx context.Context) int {
	if n, err := cpu.CountsWithContext(ctx, true); err == nil && n > 0 {
		return n
	}
	return runtime.NumCPU()
}

// readProcess snapshots one process through its persistent handle. PID is
// always populated; per-process field reads that fail (typically
// permission-restricted) fall back to the zero/empty value so the process is
// still included in the snapshot. cores normalizes CPUPercent's per-core scale
// to machine-wide 0..100.
func readProcess(ctx context.Context, p *process.Process, cores float64) ProcessInfo {
	info := ProcessInfo{PID: p.Pid}
	if name, err := p.NameWithContext(ctx); err == nil {
		info.Name = name
	}
	if cpu, err := p.CPUPercentWithContext(ctx); err == nil {
		info.CPUPercent = cpu / cores
	}
	if mem, err := p.MemoryInfoWithContext(ctx); err == nil && mem != nil {
		info.MemoryBytes = mem.RSS
	}
	if user, err := p.UsernameWithContext(ctx); err == nil {
		info.Username = user
	}
	info.State = readState(ctx, p, info.CPUPercent)
	return info
}

// readState resolves a process's coarse state. gopsutil has no status support
// on Windows (ErrNotImplementedError), so when the OS read yields nothing the
// state derives from observed CPU activity instead: a process that used CPU
// during the last sample is running, otherwise sleeping. Coarse, but truthful
// to what was actually observed.
func readState(ctx context.Context, p *process.Process, cpuPercent float64) ProcState {
	if statuses, err := p.StatusWithContext(ctx); err == nil && len(statuses) > 0 {
		if s := coarseState(statuses[0]); s != "" {
			return s
		}
	}
	if cpuPercent > 0 {
		return StateRunning
	}
	return StateSleeping
}

// coarseState folds gopsutil's OS status vocabulary into the three states the
// Processes tab shows. Zombie lands in stopped — terminated-but-unreaped is
// closest to "not running" of the visible buckets. Unknown strings report
// empty so the caller can fall back to the activity heuristic.
func coarseState(s string) ProcState {
	switch s {
	case process.Running:
		return StateRunning
	case process.Sleep, process.Idle, process.Wait, process.Lock, process.Blocked:
		return StateSleeping
	case process.Stop, process.Zombie:
		return StateStopped
	default:
		return ""
	}
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
			State:      ConnState(c.Status),
			PID:        c.Pid,
		})
	}
	return out, nil
}

// protocolName maps a socket type to a protocol label. Anything other than a
// stream or datagram socket reports an empty protocol.
func protocolName(sockType uint32) Protocol {
	switch sockType {
	case syscall.SOCK_STREAM:
		return protoTCP
	case syscall.SOCK_DGRAM:
		return protoUDP
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
		listening := (c.Protocol == protoTCP && c.State == connStateListen) ||
			(c.Protocol == protoUDP && c.RemoteAddr == "")
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
	terminate   processTerminator

	mu          sync.RWMutex
	processes   []ProcessInfo
	ports       []PortInfo
	connections []ConnectionInfo
}

// NewProcessCollector builds a collector backed by gopsutil. It takes one
// initial snapshot so the accessors return live data immediately, returning an
// error when that first snapshot fails.
func NewProcessCollector(ctx context.Context, opts ...processOption) (*ProcessCollector, error) {
	c := &ProcessCollector{
		sampleProcs: newDefaultProcessSampler(),
		sampleConns: defaultConnSampler,
		terminate:   defaultProcessTerminator,
	}
	for _, opt := range opts {
		opt(c)
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

// Terminate asks the OS to end the process with the given PID. It does not
// touch the snapshots — the next Collect naturally drops the exited process.
func (c *ProcessCollector) Terminate(ctx context.Context, pid int32) error {
	return c.terminate(ctx, pid)
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
