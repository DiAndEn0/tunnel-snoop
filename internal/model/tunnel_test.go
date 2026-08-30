package model_test

import (
	"testing"
	"time"

	"github.com/DiAndEn0/tunnel-snoop/internal/model"
)

func testWildcardDetection(t *testing.T) {
	cases := []struct {
		addr     string
		wildcard bool
	}{
		{"0.0.0.0", true},
		{"::", true},
		{"127.0.0.1", false},
		{"::1", false},
		{"192.168.1.10", false},
	}

	for _, c := range cases {
		tun := model.Tunnel{LocalAddress: c.addr}
		if tun.CheckWildcard() != c.wildcard {
			t.Fatalf("expected wildcard %v for %s, got %v", c.wildcard, c.addr, tun.CheckWildcard())
		}
	}
}

func testIdleDuration(t *testing.T) {
	now := time.Now()
	tun := model.Tunnel{
		LastActive: now.Add(-30 * time.Second),
	}
	idle := tun.CalculateIdle(now)
	if idle < 29*time.Second || idle > 31*time.Second {
		t.Fatalf("unexpected idle duration: %v", idle)
	}
}

func TestModel(t *testing.T) {
	testWildcardDetection(t)
	testIdleDuration(t)
}
