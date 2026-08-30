// Package monitor maintains in-memory tunnel state across reconciliation
// passes: it discovers currently-listening tunnel processes, correlates
// established client connections, tracks byte-level I/O activity to derive
// idle time, and evicts tunnels whose process has terminated.
package monitor

import (
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

// Engine holds the last-known state for each discovered tunnel PID so that
// successive Reconcile calls can detect activity (via I/O byte deltas and
// active client counts) and compute idle durations relative to LastActive.
type Engine struct {
	cfg     Config
	mu      sync.Mutex
	tunnels map[int]*model.Tunnel
}

// portKey identifies a listening endpoint by protocol and local port so that
// client connection counts are not conflated across protocols that happen to
// share the same port number (e.g. a TCP and TCP6 listener both on :8080).
type portKey struct {
	proto model.Protocol
	port  int
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
		tunnels: make(map[int]*model.Tunnel),
	}
}

// Reconcile performs a single discovery-and-update pass: it re-parses the
// socket table and correlates listening tunnel processes, counts
// established client connections per local port, reads each tunnel
// process's cumulative I/O byte counters, and updates LastActive/IdleDuration
// accordingly. Tunnels previously tracked but no longer discovered (i.e.
// their process has terminated or is no longer listening) are evicted from
// internal state. The returned slice reflects the current, post-reconcile
// view of all live tunnels.
func (e *Engine) Reconcile(now time.Time) ([]model.Tunnel, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	sockets, err := procfs.ParseSockets(e.cfg.NetRoot)
	if err != nil {
		return nil, err
	}

	discovered, err := procfs.FindTunnels(e.cfg.ProcRoot, sockets, e.cfg.AllowedBinaries)
	if err != nil {
		return nil, err
	}

	// Count active established clients targeting each (protocol, local port)
	// pair, so a TCP and TCP6 listener sharing a port number don't have their
	// client counts conflated.
	clientCounts := make(map[portKey]int)
	for _, s := range sockets {
		if s.State == model.StateEstablished {
			clientCounts[portKey{proto: s.Protocol, port: s.LocalPort}]++
		}
	}

	activePIDs := make(map[int]bool)
	result := make([]model.Tunnel, 0, len(discovered))

	for _, d := range discovered {
		activePIDs[d.PID] = true
		cached, exists := e.tunnels[d.PID]

		activeClients := clientCounts[portKey{proto: d.Protocol, port: d.LocalPort}]
		readBytes, writeBytes, _ := procfs.ReadProcessIO(e.cfg.ProcRoot, d.PID)

		if !exists {
			d.FirstSeen = now
			d.LastActive = now
			d.ActiveClients = activeClients
			d.BytesRead = readBytes
			d.BytesWritten = writeBytes
			d.IdleDuration = 0
			e.tunnels[d.PID] = &d
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

	// Evict tunnels whose process has vanished or is no longer listening.
	for pid := range e.tunnels {
		if !activePIDs[pid] {
			delete(e.tunnels, pid)
		}
	}

	return result, nil
}
