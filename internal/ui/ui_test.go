package ui_test

import (
	"strings"
	"testing"
	"time"

	"github.com/DiAndEn0/tunnel-snoop/internal/model"
	"github.com/DiAndEn0/tunnel-snoop/internal/ui"
)

func TestUI(t *testing.T) {
	tunnels := []model.Tunnel{
		{
			PID:          1234,
			ProcessName:  "kubectl",
			LocalAddress: "0.0.0.0",
			LocalPort:    5432,
			IsWildcard:   true,
			IdleDuration: 12 * time.Minute,
		},
	}

	table := ui.RenderTable(tunnels)
	if !strings.Contains(table, "5432") || !strings.Contains(table, "0.0.0.0") {
		t.Fatalf("table missing key data: %s", table)
	}

	jsonBytes, err := ui.FormatJSON(tunnels)
	if err != nil || !strings.Contains(string(jsonBytes), `"is_wildcard": true`) {
		t.Fatalf("json format failed: %v, output: %s", err, string(jsonBytes))
	}
}
