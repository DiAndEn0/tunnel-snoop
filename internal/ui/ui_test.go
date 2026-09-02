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
			Exposure:     model.ExposureWildcard,
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
	if !strings.Contains(string(jsonBytes), `"exposure": "wildcard"`) {
		t.Fatalf("json output missing the exposure tier: %s", string(jsonBytes))
	}
}

const (
	ansiReset  = "\033[0m"
	ansiRed    = "\033[31;1m"
	ansiGreen  = "\033[32m"
	ansiYellow = "\033[33m"
)

// TestRenderTableSecurityBadges pins one badge per exposure tier. The wildcard
// badge text is checked verbatim because README and demo.tape both quote
// "[EXPOSED 0.0.0.0]"; changing it would silently invalidate the documentation.
func TestRenderTableSecurityBadges(t *testing.T) {
	cases := []struct {
		name     string
		tunnel   model.Tunnel
		wantHave string
	}{
		{
			name:     "loopback is the only safe tier",
			tunnel:   model.Tunnel{PID: 1, ProcessName: "kubectl", LocalAddress: "127.0.0.1", LocalPort: 5432, Exposure: model.ExposureLoopback},
			wantHave: ansiGreen + "[SAFE]" + ansiReset,
		},
		{
			name:     "lan bind is warned about in yellow",
			tunnel:   model.Tunnel{PID: 2, ProcessName: "kubectl", LocalAddress: "192.168.1.40", LocalPort: 5432, Exposure: model.ExposureLAN},
			wantHave: ansiYellow + "[EXPOSED LAN]" + ansiReset,
		},
		{
			name:     "public bind is warned about in red",
			tunnel:   model.Tunnel{PID: 3, ProcessName: "kubectl", LocalAddress: "203.0.113.7", LocalPort: 5432, Exposure: model.ExposurePublic},
			wantHave: ansiRed + "[EXPOSED PUBLIC]" + ansiReset,
		},
		{
			name:     "wildcard keeps its documented wording",
			tunnel:   model.Tunnel{PID: 4, ProcessName: "kubectl", LocalAddress: "0.0.0.0", LocalPort: 5432, IsWildcard: true, Exposure: model.ExposureWildcard},
			wantHave: ansiRed + "[EXPOSED 0.0.0.0]" + ansiReset,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			table := ui.RenderTable([]model.Tunnel{c.tunnel})
			if !strings.Contains(table, c.wantHave) {
				t.Fatalf("expected badge %q in table:\n%s", c.wantHave, table)
			}
		})
	}
}

// TestRenderTableClassifiesUnsetExposure covers a Tunnel that reached the
// renderer without a classification — decoded from an older JSON payload, or
// built by a caller that skipped discovery. It must be classified from its
// binding rather than falling through to [SAFE].
func TestRenderTableClassifiesUnsetExposure(t *testing.T) {
	table := ui.RenderTable([]model.Tunnel{{PID: 5, ProcessName: "ssh", LocalAddress: "192.168.1.40", LocalPort: 22}})
	if strings.Contains(table, "[SAFE]") {
		t.Fatalf("unclassified LAN binding must not render as safe:\n%s", table)
	}
	if !strings.Contains(table, "[EXPOSED LAN]") {
		t.Fatalf("expected LAN badge for unclassified LAN binding:\n%s", table)
	}
}

// TestRenderTableHeaderAlignment keeps the rule under the header exactly as
// wide as the header itself; the SECURITY column has to fit the longest badge,
// and a stale separator width is the usual way that drifts.
func TestRenderTableHeaderAlignment(t *testing.T) {
	lines := strings.Split(ui.RenderTable(nil), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least a header and a separator, got %d lines", len(lines))
	}

	header := strings.NewReplacer("\033[1m", "", "\033[0m", "").Replace(lines[0])
	if len(header) != len(lines[1]) {
		t.Fatalf("header is %d columns wide but the separator is %d", len(header), len(lines[1]))
	}
	if !strings.Contains(header, "SECURITY") {
		t.Fatalf("expected a SECURITY column in the header: %q", header)
	}
}
