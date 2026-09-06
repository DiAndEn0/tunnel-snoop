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

var version = "dev"

// Exit codes.
const (
	exitOK      = 0
	exitExposed = 1
	exitUsage   = 2
)

func main() {
	os.Exit(run())
}

func run() int {
	interval := flag.Duration("interval", 5*time.Second, "Polling interval")
	killIdle := flag.Duration("kill-idle", 0, "Terminate tunnels idle longer than duration (e.g. 15m)")
	jsonOutput := flag.Bool("json", false, "Output in JSON format")
	once := flag.Bool("once", false, "Scan once and exit")
	port := flag.Int("port", 0, "Only report tunnels listening on this local port")
	processes := flag.String("process", "", "Only report tunnels whose process name is in this comma-separated list")
	exposedOnly := flag.Bool("exposed-only", false, "Only report tunnels flagged as exposed")
	minIdle := flag.Duration("min-idle", 0, "Only report tunnels idle at least this long (e.g. 15m)")
	failOnExposed := flag.Bool("fail-on-exposed", false, "Exit with status 1 if any exposed tunnel is found")
	showVersion := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Printf("tunnelsnoop %s\n", version)
		return exitOK
	}

	if *port < 0 || *port > 65535 {
		fmt.Fprintf(os.Stderr, "Invalid port: %d (must be between 0 and 65535)\n", *port)
		return exitUsage
	}

	if !*once && *interval <= 0 {
		fmt.Fprintf(os.Stderr, "Invalid interval: %v (must be positive duration)\n", *interval)
		return exitUsage
	}

	eng := monitor.NewEngine(monitor.Config{
		KillIdle: *killIdle,
	})

	filter := monitor.NewFilter(*port, *processes, *exposedOnly, *minIdle)

	exposureSeen := false
	scanFailed := false

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	tick := func() bool {
		now := time.Now()
		tunnels, err := eng.Reconcile(now)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error scanning tunnels: %v\n", err)
			scanFailed = true
			return false
		}

		tunnels = filter.Apply(tunnels)

		if monitor.AnyExposed(tunnels) {
			exposureSeen = true
		}

		if *killIdle > 0 {
			for _, tun := range tunnels {
				if tun.IdleDuration > *killIdle {
					fmt.Fprintf(os.Stderr, "Killing idle tunnel PID %d (%s:%d)...\n",
						tun.PID, tun.LocalAddress, tun.LocalPort)
					if err := reaper.TerminateTunnel("/proc", tun, 5*time.Second); err != nil {
						fmt.Fprintf(os.Stderr, "Failed to terminate PID %d: %v\n", tun.PID, err)
					}
				}
			}
		}

		if *jsonOutput {
			data, err := ui.FormatJSON(tunnels)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error formatting JSON: %v\n", err)
				scanFailed = true
				return false
			}
			fmt.Println(string(data))
		} else {
			if !*once {
				fmt.Print("\033[H\033[2J")
			}
			fmt.Printf("tunnelsnoop - Active Port-Forward Monitor [%s]\n\n", now.Format("15:04:05"))
			fmt.Print(ui.RenderTable(tunnels))
		}
		return true
	}

	ok := tick()
	if *once {
		if !ok {
			return exitUsage
		}
		return exitStatus(*failOnExposed, exposureSeen)
	}

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if *jsonOutput {
				fmt.Fprintln(os.Stderr, "\nShutting down tunnelsnoop...")
			} else {
				fmt.Println("\nShutting down tunnelsnoop...")
			}
			if scanFailed {
				return exitUsage
			}
			return exitStatus(*failOnExposed, exposureSeen)
		case <-ticker.C:
			tick()
		}
	}
}

func exitStatus(failOnExposed, exposureSeen bool) int {
	if failOnExposed && exposureSeen {
		return exitExposed
	}
	return exitOK
}
