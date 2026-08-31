package monitor

import (
	"testing"
	"time"
)

// NewEngine's defaults decide where the engine reads process state from and
// which binaries are eligible for termination, so they are asserted directly
// rather than through Reconcile (which would touch the host's real /proc).
func TestNewEngineAppliesDefaults(t *testing.T) {
	e := NewEngine(Config{})

	if e.cfg.ProcRoot != "/proc" {
		t.Fatalf("expected default ProcRoot /proc, got %q", e.cfg.ProcRoot)
	}
	if e.cfg.NetRoot != "/proc" {
		t.Fatalf("expected default NetRoot /proc, got %q", e.cfg.NetRoot)
	}

	want := []string{"kubectl", "ssh", "cloudflared", "ngrok"}
	if len(e.cfg.AllowedBinaries) != len(want) {
		t.Fatalf("expected %d default binaries, got %v", len(want), e.cfg.AllowedBinaries)
	}
	for i, name := range want {
		if e.cfg.AllowedBinaries[i] != name {
			t.Fatalf("expected default binary %q at index %d, got %q", name, i, e.cfg.AllowedBinaries[i])
		}
	}

	if e.tunnels == nil {
		t.Fatalf("expected tunnels map to be initialised")
	}
}

func TestNewEngineKeepsExplicitConfig(t *testing.T) {
	cfg := Config{
		ProcRoot:        "/fake/proc",
		NetRoot:         "/fake/net",
		AllowedBinaries: []string{"socat"},
		KillIdle:        90 * time.Second,
	}
	e := NewEngine(cfg)

	if e.cfg.ProcRoot != "/fake/proc" || e.cfg.NetRoot != "/fake/net" {
		t.Fatalf("explicit roots were overwritten: %+v", e.cfg)
	}
	if len(e.cfg.AllowedBinaries) != 1 || e.cfg.AllowedBinaries[0] != "socat" {
		t.Fatalf("explicit allowlist was overwritten: %v", e.cfg.AllowedBinaries)
	}
	if e.cfg.KillIdle != 90*time.Second {
		t.Fatalf("unexpected KillIdle: %v", e.cfg.KillIdle)
	}
}
