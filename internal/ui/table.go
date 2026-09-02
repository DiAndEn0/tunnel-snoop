package ui

import (
	"fmt"
	"strings"

	"github.com/DiAndEn0/tunnel-snoop/internal/model"
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31;1m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBold   = "\033[1m"
)

// securityColumnWidth must hold the widest badge text, "[EXPOSED 0.0.0.0]", or
// the header stops lining up with the rows beneath it.
const securityColumnWidth = 18

// tableWidth is the sum of every column width plus the single space between
// each pair, so the rule under the header ends where the header ends.
const tableWidth = 7 + 1 + 12 + 1 + 20 + 1 + 8 + 1 + 10 + 1 + securityColumnWidth

func RenderTable(tunnels []model.Tunnel) string {
	var sb strings.Builder
	// strings.Builder never fails a write, so the Fprintf results are discarded.
	_, _ = fmt.Fprintf(&sb, "%s%-7s %-12s %-20s %-8s %-10s %-*s%s\n",
		colorBold, "PID", "PROCESS", "LOCAL BINDING", "CLIENTS", "IDLE", securityColumnWidth, "SECURITY", colorReset)
	sb.WriteString(strings.Repeat("-", tableWidth) + "\n")

	if len(tunnels) == 0 {
		sb.WriteString("No active port-forward tunnels detected.\n")
		return sb.String()
	}

	for _, tun := range tunnels {
		binding := fmt.Sprintf("%s:%d", tun.LocalAddress, tun.LocalPort)

		idleStr := tun.IdleDuration.Round(1e9).String()
		if tun.IdleDuration < 1e9 {
			idleStr = "active"
		}

		_, _ = fmt.Fprintf(&sb, "%-7d %-12s %-20s %-8d %-10s %s\n",
			tun.PID, tun.ProcessName, binding, tun.ActiveClients, idleStr, securityBadge(tun))
	}

	return sb.String()
}

// securityBadge renders the exposure tier of a tunnel's binding.
//
// Only a loopback bind earns [SAFE]. A LAN bind is reachable by every machine
// on the network, which is the case that used to be mislabelled as safe, so it
// gets its own warning rather than sharing the wildcard's wording. The wildcard
// text is verbatim "[EXPOSED 0.0.0.0]" because README and demo.tape quote it.
//
// A tunnel that arrives without a classification is classified here instead of
// defaulting, and an unrecognised tier is treated as public: every path out of
// this function that is not a proven loopback bind has to warn.
func securityBadge(tun model.Tunnel) string {
	exposure := tun.Exposure
	if exposure == "" {
		exposure = tun.CheckExposure()
	}

	switch exposure {
	case model.ExposureLoopback:
		return fmt.Sprintf("%s[SAFE]%s", colorGreen, colorReset)
	case model.ExposureLAN:
		return fmt.Sprintf("%s[EXPOSED LAN]%s", colorYellow, colorReset)
	case model.ExposureWildcard:
		return fmt.Sprintf("%s[EXPOSED 0.0.0.0]%s", colorRed, colorReset)
	default:
		return fmt.Sprintf("%s[EXPOSED PUBLIC]%s", colorRed, colorReset)
	}
}
