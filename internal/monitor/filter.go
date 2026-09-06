package monitor

import (
	"strconv"
	"strings"
	"time"

	"github.com/DiAndEn0/tunnel-snoop/internal/model"
)

// Filter holds the inclusion criteria configured on the command line. A zero
// Filter matches every tunnel.
type Filter struct {
	Port        int
	Processes   []string
	ExposedOnly bool
	MinIdle     time.Duration
}

// NewFilter parses the raw command-line flag values into a Filter ready for
// Apply.
//
// Process names are trimmed, lowercased, and empty entries are discarded so
// that trailing or duplicate commas (e.g. "-process kubectl,") do not match
// empty process names.
func NewFilter(port int, processes string, exposedOnly bool, minIdle time.Duration) Filter {
	var procs []string
	for _, p := range strings.Split(processes, ",") {
		p = strings.ToLower(strings.TrimSpace(p))
		if p != "" {
			procs = append(procs, p)
		}
	}
	return Filter{
		Port:        port,
		Processes:   procs,
		ExposedOnly: exposedOnly,
		MinIdle:     minIdle,
	}
}

// Apply returns the subset of tunnels that satisfy every criterion in f.
// An empty slice is returned when no tunnels match; nil is never returned,
// which keeps JSON encoding consistent.
func (f Filter) Apply(tunnels []model.Tunnel) []model.Tunnel {
	if f.isZero() {
		if tunnels == nil {
			return []model.Tunnel{}
		}
		return tunnels
	}

	result := make([]model.Tunnel, 0, len(tunnels))
	for _, t := range tunnels {
		if f.matches(t) {
			result = append(result, t)
		}
	}
	return result
}

// isZero reports whether f has no criteria set, in which case filtering is a
// no-op and Apply can skip scanning individual tunnels.
func (f Filter) isZero() bool {
	return f.Port == 0 &&
		len(f.Processes) == 0 &&
		!f.ExposedOnly &&
		f.MinIdle == 0
}

// IsExposed reports whether the tunnel binding is accessible beyond loopback.
func IsExposed(tunnel model.Tunnel) bool {
	exposure := tunnel.Exposure
	if exposure == "" {
		exposure = tunnel.CheckExposure()
	}
	return exposure != model.ExposureLoopback
}

// AnyExposed reports whether at least one tunnel in the slice is accessible
// beyond loopback.
func AnyExposed(tunnels []model.Tunnel) bool {
	for _, t := range tunnels {
		if IsExposed(t) {
			return true
		}
	}
	return false
}

// matches reports whether t satisfies every active criterion in f.
func (f Filter) matches(t model.Tunnel) bool {
	if f.Port != 0 && t.LocalPort != f.Port {
		return false
	}
	if len(f.Processes) > 0 && !f.matchesProcess(t.ProcessName) {
		return false
	}
	if f.ExposedOnly && !IsExposed(t) {
		return false
	}
	if f.MinIdle > 0 && t.IdleDuration < f.MinIdle {
		return false
	}
	return true
}

func (f Filter) matchesProcess(name string) bool {
	name = strings.ToLower(name)
	for _, p := range f.Processes {
		if name == p {
			return true
		}
	}
	return false
}

// MatchesPort is an exported helper so that callers who want to check a
// single tunnel against a port string without constructing a Filter can do so.
// It returns true on parse error to avoid silently dropping tunnels when given
// malformed input.
func MatchesPort(tunnelPort int, portStr string) bool {
	if portStr == "" {
		return true
	}
	p, err := strconv.Atoi(portStr)
	if err != nil {
		return true
	}
	return tunnelPort == p
}
