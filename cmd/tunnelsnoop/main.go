package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/DiAndEn0/tunnel-snoop/internal/monitor"
	"github.com/DiAndEn0/tunnel-snoop/internal/reaper"
	"github.com/DiAndEn0/tunnel-snoop/internal/ui"
)

func main() {
	interval := flag.Duration("interval", 5*time.Second, "Polling interval")
	killIdle := flag.Duration("kill-idle", 0, "Terminate tunnels idle longer than duration (e.g. 15m)")
	jsonOutput := flag.Bool("json", false, "Output in JSON format")
	once := flag.Bool("once", false, "Scan once and exit")
	flag.Parse()

	eng := monitor.NewEngine(monitor.Config{
		KillIdle: *killIdle,
	})

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	tick := func() {
		now := time.Now()
		tunnels, err := eng.Reconcile(now)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error scanning tunnels: %v\n", err)
			return
		}

		if *killIdle > 0 {
			for _, tun := range tunnels {
				if tun.IdleDuration > *killIdle {
					fmt.Fprintf(os.Stderr, "Killing idle tunnel PID %d (%s:%d)...\n",
						tun.PID, tun.LocalAddress, tun.LocalPort)
					// Report refusals: the reaper aborts rather than signalling
					// when a PID has been recycled or its socket has closed, and
					// silence there is indistinguishable from a successful kill.
					if err := reaper.TerminateTunnel("/proc", tun, 5*time.Second); err != nil {
						fmt.Fprintf(os.Stderr, "Failed to terminate PID %d: %v\n", tun.PID, err)
					}
				}
			}
		}

		if *jsonOutput {
			data, _ := ui.FormatJSON(tunnels)
			fmt.Println(string(data))
		} else {
			if !*once {
				fmt.Print("\033[H\033[2J") // Clear screen
			}
			fmt.Printf("tunnelsnoop - Active Port-Forward Monitor [%s]\n\n", now.Format("15:04:05"))
			fmt.Print(ui.RenderTable(tunnels))
		}
	}

	// Initial execution
	tick()
	if *once {
		return
	}

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("\nShutting down tunnelsnoop...")
			return
		case <-ticker.C:
			tick()
		}
	}
}
