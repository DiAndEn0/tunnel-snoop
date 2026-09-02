// Package monitor maintains in-memory tunnel state across reconciliation
// passes: it discovers currently-listening tunnel processes, correlates
// established client connections, tracks byte-level I/O activity to derive
// idle time, and evicts tunnels whose process has terminated or whose
// listening socket has closed.
package monitor

import (
	"os"
	"sync"
	"time"

	"github.com/DiAndEn0/tunnel-snoop/internal/model"
	"github.com/DiAndEn0/tunnel-snoop/internal/procfs"
)

// Config controls where the Engine looks for process and socket state, which
// binaries are considered tunnels, and how long a tunnel may sit idle before
// a caller (e.g. the reaper) should consider terminating it.
type Config struct {
	ProcRoot        string
	NetRoot         string
	AllowedBinaries []string
	KillIdle        time.Duration
}

// tunnelKey uniquely identifies a tracked tunnel endpoint by its PID and socket
// inode.
//
// Keying on SocketInode (rather than PID alone) allows a single multi-forward
// process (such as `ssh -L 8080:... -L 9090:...` or `kubectl port-forward`
// with multiple port arguments) to track and report all listening endpoints
// independently without clobbering one another.
//
// Including PID in the composite key guards against kernel socket inode
// recycling across process boundaries: if an inode is reused by a different
// process before the next reconciliation pass, it is treated as a distinct
// tunnel rather than inheriting stale I/O byte counts or activity timestamps.
type tunnelKey struct {
	pid   int
	inode uint64
}

// Engine holds the last-known state for each discovered tunnel endpoint so that
// successive Reconcile calls can detect activity (via I/O byte deltas and
// active client counts) and compute idle durations relative to LastActive.
type Engine struct {
	cfg     Config
	mu      sync.Mutex
	tunnels map[tunnelKey]*model.Tunnel
}

// portKey identifies a listening endpoint by protocol and local port so that
// client connection counts are not conflated across protocols that happen to
// share the same port number (e.g. a TCP and TCP6 listener both on :8080).
type portKey struct {
	proto model.Protocol
	port  int
}

// countClients returns the number of established connections attributable to
// tunnel, given per-endpoint counts keyed by protocol, port and local IP.
//
// A wildcard listener accepts on every interface, and the resulting connections
// carry the specific interface address rather than 0.0.0.0, so every local IP on
// the port counts. A listener bound to one address counts only connections
// terminating on that address.
func countClients(counts map[portKey]map[string]int, tunnel model.Tunnel) int {
	byIP := counts[portKey{proto: tunnel.Protocol, port: tunnel.LocalPort}]
	if byIP == nil {
		return 0
	}

	if tunnel.IsWildcard {
		total := 0
		for _, n := range byIP {
			total += n
		}
		return total
	}

	return byIP[tunnel.LocalAddress]
}

// NewEngine constructs an Engine with the given Config, applying sane
// defaults for any zero-valued fields.
func NewEngine(cfg Config) *Engine {
	if cfg.ProcRoot == "" {
		cfg.ProcRoot = "/proc"
	}
	if cfg.NetRoot == "" {
		cfg.NetRoot = "/proc"
	}
	if len(cfg.AllowedBinaries) == 0 {
		cfg.AllowedBinaries = []string{"kubectl", "ssh", "cloudflared", "ngrok"}
	}
	return &Engine{
		cfg:     cfg,
		tunnels: make(map[tunnelKey]*model.Tunnel),
	}
}

// Reconcile performs a single discovery-and-update pass: it re-parses the
// socket table and correlates listening tunnel processes, counts
// established client connections per local port, reads each tunnel
// process's cumulative I/O byte counters, and updates LastActive/IdleDuration
// accordingly. Tunnels previously tracked but no longer discovered (i.e.
// their process has terminated or their specific listening socket has closed)
// are evicted from internal state. The returned slice reflects the current,
// post-reconcile view of all live tunnels.
func (e *Engine) Reconcile(now time.Time) ([]model.Tunnel, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	sockets, err := procfs.ParseSockets(e.cfg.NetRoot)
	if err != nil {
		return nil, err
	}

	discovered, err := procfs.FindTunnels(e.cfg.ProcRoot, sockets, e.cfg.AllowedBinaries, os.Getuid())
	if err != nil {
		return nil, err
	}

	// Count established connections per (protocol, local port, local IP). The
	// protocol keeps a TCP and TCP6 listener on the same port number distinct;
	// the local IP keeps a loopback-bound tunnel from absorbing connections
	// terminating on an external interface, or an unrelated outbound connection
	// that happens to have been assigned a matching ephemeral local port.
	clientCounts := make(map[portKey]map[string]int)
	for _, s := range sockets {
		if s.State != model.StateEstablished {
			continue
		}
		key := portKey{proto: s.Protocol, port: s.LocalPort}
		if clientCounts[key] == nil {
			clientCounts[key] = make(map[string]int)
		}
		clientCounts[key][s.LocalIP]++
	}

	activeKeys := make(map[tunnelKey]bool)
	result := make([]model.Tunnel, 0, len(discovered))

	for _, d := range discovered {
		key := tunnelKey{pid: d.PID, inode: d.SocketInode}
		activeKeys[key] = true
		cached, exists := e.tunnels[key]

		activeClients := countClients(clientCounts, d)
		readBytes, writeBytes, _ := procfs.ReadProcessIO(e.cfg.ProcRoot, d.PID)

		if !exists {
			d.FirstSeen = now
			d.LastActive = now
			d.ActiveClients = activeClients
			d.BytesRead = readBytes
			d.BytesWritten = writeBytes
			d.IdleDuration = 0
			e.tunnels[key] = &d
			result = append(result, d)
		} else {
			cached.ActiveClients = activeClients
			ioChanged := (readBytes != cached.BytesRead) || (writeBytes != cached.BytesWritten)
			if activeClients > 0 || ioChanged {
				cached.LastActive = now
				cached.BytesRead = readBytes
				cached.BytesWritten = writeBytes
			}
			cached.IdleDuration = cached.CalculateIdle(now)
			result = append(result, *cached)
		}
	}

	// Evict tunnels whose process vanished or whose socket closed.
	for key := range e.tunnels {
		if !activeKeys[key] {
			delete(e.tunnels, key)
		}
	}

	return result, nil
}
