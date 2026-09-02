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

// version is the release this binary was built from. Release builds override it
// with -ldflags "-X main.version=<tag>"; unstamped builds report "dev".
var version = "dev"

// Exit codes. These are part of the command's contract with scripts and CI
// steps, so they are named here and documented in the man page's EXIT STATUS
// section rather than written as bare integers at the return sites.
const (
	// exitOK reports a completed run in which no exposure had to be flagged.
	exitOK = 0

	// exitExposed reports that -fail-on-exposed was set and at least one
	// exposed tunnel was seen, which is what makes the command usable as a
	// gate in a CI job or a pre-commit hook.
	exitExposed = 1

	// exitUsage reports an invalid command line or operational failure during
	// execution (e.g. procfs/net socket tables unreadable).
	exitUsage = 2
)

func main() {
	os.Exit(run())
}

// run carries the entire command so that main is nothing but a single
// os.Exit. os.Exit does not run deferred functions, so returning the status up
// to main is what keeps the signal-context cancel and the ticker stop below
// from being skipped on the exit paths.
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

	eng := monitor.NewEngine(monitor.Config{
		KillIdle: *killIdle,
	})

	filter := monitor.NewFilter(*port, *processes, *exposedOnly, *minIdle)

	// Whether an exposure was seen at any point. In continuous mode the
	// exposure that should fail the run may appear in a pass long before the
	// operator interrupts the monitor, and a tunnel that has since been closed
	// (or reaped) was no less exposed while it was open, so the observation is
	// remembered rather than re-derived from the final pass.
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

		// Filter before anything downstream looks at the set, so the reaper
		// acts on exactly what the operator asked to see. "-process kubectl
		// -kill-idle 15m" is the useful reading of that pair, and reaping
		// tunnels excluded from the display would be a destructive surprise.
		tunnels = filter.Apply(tunnels)

		if monitor.AnyExposed(tunnels) {
			exposureSeen = true
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
		return true
	}

	// Initial execution
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
			fmt.Println("\nShutting down tunnelsnoop...")
			if scanFailed && !exposureSeen {
				return exitUsage
			}
			return exitStatus(*failOnExposed, exposureSeen)
		case <-ticker.C:
			tick()
		}
	}
}

// exitStatus maps an observed exposure onto the process exit code. Without
// -fail-on-exposed the exposure is reported in the output only, keeping the
// exit code of an ordinary run identical to what it has always been.
func exitStatus(failOnExposed, exposureSeen bool) int {
	if failOnExposed && exposureSeen {
		return exitExposed
	}
	return exitOK
}
