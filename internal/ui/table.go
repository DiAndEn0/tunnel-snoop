package ui

import (
	"fmt"
	"strings"

	"github.com/DiAndEn0/tunnel-snoop/internal/model"
)

const (
	colorReset = "\033[0m"
	colorRed   = "\033[31;1m"
	colorGreen = "\033[32m"
	colorBold  = "\033[1m"
)

func RenderTable(tunnels []model.Tunnel) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s%-7s %-12s %-20s %-8s %-10s %-10s%s\n",
		colorBold, "PID", "PROCESS", "LOCAL BINDING", "CLIENTS", "IDLE", "SECURITY", colorReset))
	sb.WriteString(strings.Repeat("-", 75) + "\n")

	if len(tunnels) == 0 {
		sb.WriteString("No active port-forward tunnels detected.\n")
		return sb.String()
	}

	for _, tun := range tunnels {
		binding := fmt.Sprintf("%s:%d", tun.LocalAddress, tun.LocalPort)
		secBadge := fmt.Sprintf("%s[SAFE]%s", colorGreen, colorReset)
		if tun.IsWildcard {
			secBadge = fmt.Sprintf("%s[EXPOSED 0.0.0.0]%s", colorRed, colorReset)
		}

		idleStr := tun.IdleDuration.Round(1e9).String()
		if tun.IdleDuration < 1e9 {
			idleStr = "active"
		}

		sb.WriteString(fmt.Sprintf("%-7d %-12s %-20s %-8d %-10s %s\n",
			tun.PID, tun.ProcessName, binding, tun.ActiveClients, idleStr, secBadge))
	}

	return sb.String()
}
