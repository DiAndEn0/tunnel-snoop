package monitor

import (
	"strings"
	"time"

	"github.com/DiAndEn0/tunnel-snoop/internal/model"
)

// Filter narrows a reconciled tunnel set to the tunnels an operator asked
// about. It is applied once per pass, between reconciliation and everything
// downstream, so the narrowed set is both what is displayed and what the
// reaper acts on: "-process kubectl -kill-idle 15m" terminates idle kubectl
// tunnels and leaves an idle ssh tunnel running. Filtering the display alone
// would make that combination silently reap tunnels the operator had just
// excluded from view.
//
// The zero value selects everything, so a flag left unset costs nothing.
type Filter struct {
	// Port selects tunnels listening on this local port; 0 disables the
	// predicate. Port 0 is never a real listening port, so it doubles as the
	// "unset" sentinel without a second field.
	Port int

	// Processes holds lower-cased process names to match ProcessName against
	// exactly. Matching is exact rather than substring so that "-process ssh"
	// cannot pull in an unrelated "sshuttle" tunnel and hand it to the reaper.
	// An empty slice disables the predicate.
	Processes []string

	// ExposedOnly restricts the set to tunnels IsExposed reports on.
	ExposedOnly bool

	// MinIdle selects tunnels idle for at least this long. The comparison is
	// inclusive, which is how "-min-idle 20m" reads; note that -kill-idle is
	// deliberately exclusive, being a destructive threshold rather than a
	// reporting one.
	MinIdle time.Duration
}

// NewFilter builds a Filter from raw flag values, parsing processes as the
// comma-separated list accepted by -process. Keeping the parsing here rather
// than in the command lets the list handling be tested in isolation and keeps
// main.go to flag declaration and wiring.
func NewFilter(port int, processes string, exposedOnly bool, minIdle time.Duration) Filter {
	f := Filter{
		Port:        port,
		ExposedOnly: exposedOnly,
		MinIdle:     minIdle,
	}

	for _, name := range strings.Split(processes, ",") {
		name = strings.ToLower(strings.TrimSpace(name))
		// Drop empty entries so that "-process ''" and a trailing comma leave
		// the predicate disabled instead of producing a name no process can
		// ever match, which would silently report zero tunnels.
		if name != "" {
			f.Processes = append(f.Processes, name)
		}
	}

	return f
}

// IsExposed reports whether a tunnel's binding exposes it beyond the local
// host. It is the single predicate the exposure-related behaviour of the tool
// (the -exposed-only filter and the -fail-on-exposed gate) is built on.
//
// A richer exposure severity model is landing on the sibling branch
// feat/activity-exposure-accuracy; this function is the intended integration
// point for it, so it is kept as one small predicate that the merge can
// rewrite in a single line rather than a condition spread over call sites.
func IsExposed(tunnel model.Tunnel) bool {
	exposure := tunnel.Exposure
	if exposure == "" {
		exposure = tunnel.CheckExposure()
	}
	return exposure != model.ExposureLoopback
}

// AnyExposed reports whether tunnels holds at least one exposed tunnel.
func AnyExposed(tunnels []model.Tunnel) bool {
	for _, tunnel := range tunnels {
		if IsExposed(tunnel) {
			return true
		}
	}
	return false
}

// Apply returns the tunnels matching every configured predicate. Predicates
// combine with AND: each flag narrows what the previous ones left.
//
// The result is always a freshly allocated, non-nil slice. Non-nil matters
// because ui.FormatJSON marshals a nil slice as "null" rather than "[]", which
// would break any consumer parsing the -json output as an array; freshly
// allocated keeps the engine's reconciled view intact for the caller.
func (f Filter) Apply(tunnels []model.Tunnel) []model.Tunnel {
	result := make([]model.Tunnel, 0, len(tunnels))

	for _, tunnel := range tunnels {
		if f.matches(tunnel) {
			result = append(result, tunnel)
		}
	}

	return result
}

// matches reports whether a single tunnel satisfies every active predicate.
func (f Filter) matches(tunnel model.Tunnel) bool {
	if f.Port != 0 && tunnel.LocalPort != f.Port {
		return false
	}
	if len(f.Processes) > 0 && !f.matchesProcess(tunnel.ProcessName) {
		return false
	}
	if f.ExposedOnly && !IsExposed(tunnel) {
		return false
	}
	if f.MinIdle > 0 && tunnel.IdleDuration < f.MinIdle {
		return false
	}
	return true
}

// matchesProcess reports whether name is one of the requested process names.
// Comparison is case-insensitive because the names come from a human on the
// command line, while ProcessName comes verbatim from /proc/<pid>/comm.
func (f Filter) matchesProcess(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	for _, want := range f.Processes {
		if name == want {
			return true
		}
	}
	return false
}
