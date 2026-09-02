package monitor

import (
	"testing"
	"time"

	"github.com/DiAndEn0/tunnel-snoop/internal/model"
)

// sample is the fixed tunnel set the filter cases below select from: two
// kubectl tunnels differing in port, exposure and idle time, plus one ssh
// tunnel, which is enough for every predicate to be able to discriminate.
func sample() []model.Tunnel {
	return []model.Tunnel{
		{
			PID:          101,
			ProcessName:  "kubectl",
			LocalAddress: "127.0.0.1",
			LocalPort:    5432,
			IdleDuration: 30 * time.Second,
		},
		{
			PID:          102,
			ProcessName:  "kubectl",
			LocalAddress: "0.0.0.0",
			LocalPort:    6379,
			IsWildcard:   true,
			IdleDuration: 20 * time.Minute,
		},
		{
			PID:          103,
			ProcessName:  "ssh",
			LocalAddress: "127.0.0.1",
			LocalPort:    6379,
			IdleDuration: 0,
		},
	}
}

func pids(tunnels []model.Tunnel) []int {
	out := make([]int, 0, len(tunnels))
	for _, t := range tunnels {
		out = append(out, t.PID)
	}
	return out
}

func assertPIDs(t *testing.T, got []model.Tunnel, want ...int) {
	t.Helper()

	gotPIDs := pids(got)
	if len(gotPIDs) != len(want) {
		t.Fatalf("expected PIDs %v, got %v", want, gotPIDs)
	}
	for i, pid := range want {
		if gotPIDs[i] != pid {
			t.Fatalf("expected PIDs %v, got %v", want, gotPIDs)
		}
	}
}

func TestFilterZeroValuePassesEverythingThrough(t *testing.T) {
	assertPIDs(t, Filter{}.Apply(sample()), 101, 102, 103)
}

func TestFilterByPort(t *testing.T) {
	assertPIDs(t, Filter{Port: 6379}.Apply(sample()), 102, 103)
}

func TestFilterByProcessNameIsCaseInsensitive(t *testing.T) {
	assertPIDs(t, NewFilter(0, "KUBECTL", false, 0).Apply(sample()), 101, 102)
}

// A substring match would let "kube" or "ssh" select tunnels the operator did
// not name, which matters because the filtered set is also what -kill-idle
// reaps.
func TestFilterByProcessNameRejectsSubstrings(t *testing.T) {
	assertPIDs(t, NewFilter(0, "kube", false, 0).Apply(sample()))
	assertPIDs(t, NewFilter(0, "kubectl-proxy", false, 0).Apply(sample()))
}

func TestFilterByProcessNameAcceptsAList(t *testing.T) {
	assertPIDs(t, NewFilter(0, "ssh, kubectl", false, 0).Apply(sample()), 101, 102, 103)
}

// An empty -process value, and a value that is nothing but separators, must
// disable the predicate rather than match no tunnel at all.
func TestFilterByProcessNameIgnoresEmptyEntries(t *testing.T) {
	assertPIDs(t, NewFilter(0, "", false, 0).Apply(sample()), 101, 102, 103)
	assertPIDs(t, NewFilter(0, " , ", false, 0).Apply(sample()), 101, 102, 103)
}

func TestFilterExposedOnly(t *testing.T) {
	assertPIDs(t, Filter{ExposedOnly: true}.Apply(sample()), 102)
}

// MinIdle is inclusive: a tunnel idle for exactly the requested duration is
// reported, matching how an operator reads "-min-idle 20m".
func TestFilterMinIdleIsInclusive(t *testing.T) {
	assertPIDs(t, Filter{MinIdle: 20 * time.Minute}.Apply(sample()), 102)
	assertPIDs(t, Filter{MinIdle: 20*time.Minute + time.Second}.Apply(sample()))
}

func TestFilterCombinesPredicatesWithAnd(t *testing.T) {
	f := NewFilter(6379, "kubectl", true, 10*time.Minute)
	assertPIDs(t, f.Apply(sample()), 102)

	// Every predicate must be able to veto on its own.
	assertPIDs(t, NewFilter(5432, "kubectl", true, 0).Apply(sample()))
	assertPIDs(t, NewFilter(6379, "ssh", true, 0).Apply(sample()))
	assertPIDs(t, NewFilter(6379, "kubectl", true, time.Hour).Apply(sample()))
}

// Apply must not hand back a nil slice: ui.FormatJSON marshals nil as "null"
// rather than "[]", which would break consumers (and the integration test)
// that parse the -json output as an array.
func TestFilterApplyNeverReturnsNil(t *testing.T) {
	if got := (Filter{Port: 1}).Apply(sample()); got == nil {
		t.Fatalf("expected an empty slice, got nil")
	}
	if got := (Filter{}).Apply(nil); got == nil {
		t.Fatalf("expected an empty slice, got nil")
	}
}

// Apply returns a new slice so that a caller holding the reconciled set (the
// engine's own view of live tunnels) is unaffected by the narrowing.
func TestFilterApplyDoesNotMutateInput(t *testing.T) {
	in := sample()
	Filter{Port: 6379}.Apply(in)
	assertPIDs(t, in, 101, 102, 103)
}

func TestIsExposedFollowsWildcardBinding(t *testing.T) {
	if IsExposed(model.Tunnel{LocalAddress: "127.0.0.1"}) {
		t.Fatalf("loopback binding must not be reported as exposed")
	}
	if !IsExposed(model.Tunnel{LocalAddress: "0.0.0.0", IsWildcard: true}) {
		t.Fatalf("wildcard binding must be reported as exposed")
	}
}

func TestAnyExposed(t *testing.T) {
	if AnyExposed(nil) {
		t.Fatalf("an empty set cannot contain an exposure")
	}
	if AnyExposed(Filter{Port: 5432}.Apply(sample())) {
		t.Fatalf("the loopback-only set must not report an exposure")
	}
	if !AnyExposed(sample()) {
		t.Fatalf("expected the wildcard tunnel to be reported")
	}
}
