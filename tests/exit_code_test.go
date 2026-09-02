package tests

import (
	"bufio"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// buildTunnelsnoop builds the command under test into a temp dir the test owns
// and returns its path.
func buildTunnelsnoop(t *testing.T) string {
	t.Helper()

	binary := filepath.Join(t.TempDir(), "tunnelsnoop")
	cmd := exec.Command(goTool(t), "build", "-o", binary, "../cmd/tunnelsnoop")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build tunnelsnoop: %v, out: %s", err, string(out))
	}
	return binary
}

// startExposedTunnel starts the faketunnel helper under a name tunnelsnoop
// discovers ("ssh") and returns the wildcard port it is listening on. The
// helper is stopped when the test ends.
//
// A real listener is used rather than a synthetic /proc tree because the exit
// code is a property of the whole command, and the discovery path it depends
// on reads the host's /proc.
func startExposedTunnel(t *testing.T) int {
	t.Helper()

	binary := filepath.Join(t.TempDir(), "ssh")
	build := exec.Command(goTool(t), "build", "-o", binary, "./testdata/faketunnel")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("failed to build faketunnel: %v, out: %s", err, string(out))
	}

	cmd := exec.Command(binary)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("failed to pipe faketunnel stdout: %v", err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start faketunnel: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil {
		t.Fatalf("faketunnel did not report its port: %v", err)
	}
	port, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil {
		t.Fatalf("faketunnel reported an unparsable port %q: %v", line, err)
	}
	return port
}

// runExit runs the command and returns its exit code and combined output. A
// non-zero exit is expected in most cases here, so it is reported rather than
// failing the test.
func runExit(t *testing.T, binary string, args ...string) (int, string) {
	t.Helper()

	cmd := exec.Command(binary, args...)
	out, err := cmd.CombinedOutput()

	// A non-zero exit is the expected outcome for most cases here; anything
	// that is not an exit status (a failed start, for instance) is a test bug.
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		t.Fatalf("failed to run tunnelsnoop: %v, out: %s", err, string(out))
	}

	return cmd.ProcessState.ExitCode(), string(out)
}

func TestFailOnExposedExitsOneWhenAnExposureIsFound(t *testing.T) {
	port := startExposedTunnel(t)
	binary := buildTunnelsnoop(t)

	code, out := runExit(t, binary, "-once", "-port", strconv.Itoa(port), "-fail-on-exposed")
	if code != 1 {
		t.Fatalf("expected exit code 1 for an exposed tunnel, got %d, out: %s", code, out)
	}
}

// The gate reports on the filtered set, so a filter that excludes the exposure
// must leave the exit code clean; otherwise "-process X -fail-on-exposed"
// would fail CI over a tunnel the operator did not ask about.
func TestFailOnExposedExitsZeroWhenTheFilterExcludesTheExposure(t *testing.T) {
	startExposedTunnel(t)
	binary := buildTunnelsnoop(t)

	code, out := runExit(t, binary, "-once", "-process", "no-such-process", "-fail-on-exposed")
	if code != 0 {
		t.Fatalf("expected exit code 0 when no tunnel matches, got %d, out: %s", code, out)
	}
}

// Without the flag the exit code must stay what it has always been, even with
// an exposed tunnel present.
func TestExposureWithoutTheGateStillExitsZero(t *testing.T) {
	port := startExposedTunnel(t)
	binary := buildTunnelsnoop(t)

	code, out := runExit(t, binary, "-once", "-port", strconv.Itoa(port))
	if code != 0 {
		t.Fatalf("expected exit code 0 without -fail-on-exposed, got %d, out: %s", code, out)
	}
}

// The gate changes the exit code only; the report is still written in full so
// that a failing CI step shows what tripped it.
func TestFailOnExposedStillEmitsItsReport(t *testing.T) {
	port := startExposedTunnel(t)
	binary := buildTunnelsnoop(t)

	code, out := runExit(t, binary, "-once", "-json", "-port", strconv.Itoa(port), "-fail-on-exposed")
	if code != 1 {
		t.Fatalf("expected exit code 1 for an exposed tunnel, got %d, out: %s", code, out)
	}
	if !strings.HasPrefix(strings.TrimSpace(out), "[") {
		t.Fatalf("expected JSON array output, got: %s", out)
	}
	if !strings.Contains(out, `"is_wildcard": true`) {
		t.Fatalf("expected the exposed tunnel in the report, got: %s", out)
	}
}

// In continuous mode the exposure is observed during the loop but the exit
// code can only be applied when the loop is interrupted, so the observation
// has to survive until then.
func TestFailOnExposedAppliesToInterruptedMonitorLoop(t *testing.T) {
	port := startExposedTunnel(t)
	binary := buildTunnelsnoop(t)

	cmd := exec.Command(binary, "-interval", "200ms", "-port", strconv.Itoa(port), "-fail-on-exposed")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start tunnelsnoop: %v", err)
	}

	// One full scan is enough to record the exposure; the initial pass runs
	// before the first tick.
	time.Sleep(time.Second)
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatalf("failed to interrupt tunnelsnoop: %v", err)
	}

	_ = cmd.Wait()
	if code := cmd.ProcessState.ExitCode(); code != 1 {
		t.Fatalf("expected exit code 1 after interrupting a monitor that saw an exposure, got %d", code)
	}
}
