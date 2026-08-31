package reaper_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/DiAndEn0/tunnel-snoop/internal/model"
	"github.com/DiAndEn0/tunnel-snoop/internal/reaper"
)

// startSleeper launches a long-lived child process and registers cleanup so the
// test never leaks it, regardless of whether the reaper signalled it.
func startSleeper(t *testing.T) *exec.Cmd {
	t.Helper()
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start test process: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	return cmd
}

// fakeProcRoot builds a synthetic /proc tree containing only <pid>/comm, so the
// reaper's identity checks can be driven without racing a real process table.
func fakeProcRoot(t *testing.T, pid int, comm string) string {
	t.Helper()
	root := t.TempDir()
	pidDir := filepath.Join(root, strconv.Itoa(pid))
	if err := os.MkdirAll(pidDir, 0o755); err != nil {
		t.Fatalf("failed to create fake proc dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pidDir, "comm"), []byte(comm+"\n"), 0o644); err != nil {
		t.Fatalf("failed to write fake comm: %v", err)
	}
	return root
}

// assertAlive fails the test if the process has already been reaped, proving an
// abort path returned before any signal was delivered.
func assertAlive(t *testing.T, cmd *exec.Cmd) {
	t.Helper()
	if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
		t.Fatalf("expected process to still be alive after aborted kill: %v", err)
	}
}

func TestReaperTermination_AbortsWhenPIDReused(t *testing.T) {
	cmd := startSleeper(t)

	// comm reports a different binary than the one discovery recorded, which is
	// what a recycled PID looks like from the reaper's perspective.
	procRoot := fakeProcRoot(t, cmd.Process.Pid, "nginx")

	tun := model.Tunnel{
		PID:         cmd.Process.Pid,
		ProcessName: "sleep",
	}

	err := reaper.TerminateTunnel(procRoot, tun, 200*time.Millisecond)
	if err == nil {
		t.Fatalf("expected error due to PID reuse, got nil")
	}
	if !strings.Contains(err.Error(), "reused") {
		t.Fatalf("expected PID reuse error, got: %v", err)
	}

	assertAlive(t, cmd)
}

func TestReaperTermination_MatchesCommCaseInsensitively(t *testing.T) {
	cmd := startSleeper(t)
	procRoot := fakeProcRoot(t, cmd.Process.Pid, "SLEEP")

	// Reap the child as soon as it dies. Without this the exited process lingers
	// as a zombie, signal 0 keeps succeeding, and the grace-period loop runs to
	// exhaustion instead of returning early on clean exit.
	reaped := make(chan struct{})
	go func() {
		_, _ = cmd.Process.Wait()
		close(reaped)
	}()

	if err := reaper.TerminateTunnel(procRoot, tun(cmd.Process.Pid), 3*time.Second); err != nil {
		t.Fatalf("expected case-insensitive comm match to proceed, got: %v", err)
	}

	select {
	case <-reaped:
	case <-time.After(2 * time.Second):
		t.Fatalf("process was not terminated")
	}
}

// tun builds the minimal Tunnel the reaper needs for a "sleep" process with no
// socket verification.
func tun(pid int) model.Tunnel {
	return model.Tunnel{PID: pid, ProcessName: "sleep"}
}

func TestReaperTermination_AbortsWhenCommUnreadable(t *testing.T) {
	cmd := startSleeper(t)

	// An empty proc root: no <pid>/comm exists, mimicking a process that exited
	// between discovery and termination.
	tun := model.Tunnel{
		PID:         cmd.Process.Pid,
		ProcessName: "sleep",
	}

	err := reaper.TerminateTunnel(t.TempDir(), tun, 200*time.Millisecond)
	if err == nil {
		t.Fatalf("expected error for unreadable comm, got nil")
	}
	if !strings.Contains(err.Error(), "already exited or unreadable") {
		t.Fatalf("expected unreadable-comm error, got: %v", err)
	}

	assertAlive(t, cmd)
}

func TestReaperTermination_AbortsWhenFDDirectoryUnreadable(t *testing.T) {
	cmd := startSleeper(t)

	// comm matches, but the synthetic proc tree has no <pid>/fd directory, so
	// socket verification cannot confirm ownership.
	procRoot := fakeProcRoot(t, cmd.Process.Pid, "sleep")

	tun := model.Tunnel{
		PID:         cmd.Process.Pid,
		ProcessName: "sleep",
		SocketInode: 4242,
	}

	err := reaper.TerminateTunnel(procRoot, tun, 200*time.Millisecond)
	if err == nil {
		t.Fatalf("expected error for unreadable fd directory, got nil")
	}
	if !strings.Contains(err.Error(), "cannot read fd directory") {
		t.Fatalf("expected fd directory error, got: %v", err)
	}

	assertAlive(t, cmd)
}

// The identity checks must be repeated before SIGKILL, not just before SIGTERM.
// The target is not our child, so it may exit during the grace period and have
// its PID recycled; signal 0 then succeeds against an unrelated process and the
// escalation would otherwise kill it. Rewriting comm mid-grace reproduces that
// state deterministically.
func TestReaperTermination_AbortsEscalationWhenPIDReusedDuringGrace(t *testing.T) {
	// Ignoring SIGTERM guarantees the reaper reaches the escalation path.
	cmd := exec.Command("sh", "-c", "trap '' TERM; sleep 30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start test process: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})

	pid := cmd.Process.Pid
	procRoot := fakeProcRoot(t, pid, "sleep")
	commPath := filepath.Join(procRoot, strconv.Itoa(pid), "comm")

	// Swap comm partway through the grace period, as a recycled PID would.
	go func() {
		time.Sleep(150 * time.Millisecond)
		_ = os.WriteFile(commPath, []byte("nginx\n"), 0o644)
	}()

	err := reaper.TerminateTunnel(procRoot, tun(pid), 600*time.Millisecond)
	if err == nil {
		t.Fatalf("expected escalation to abort after PID reuse, got nil")
	}
	if !strings.Contains(err.Error(), "aborting SIGKILL escalation") {
		t.Fatalf("expected escalation abort error, got: %v", err)
	}

	assertAlive(t, cmd)
}

func TestReaperTermination_EscalatesToSIGKILL(t *testing.T) {
	// A shell that ignores SIGTERM outlives the grace period, forcing the
	// reaper down its escalation path.
	cmd := exec.Command("sh", "-c", "trap '' TERM; sleep 30")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start test process: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	pid := cmd.Process.Pid

	// Read the real comm rather than assuming "sh": the shell may be dash, bash
	// or busybox depending on the host.
	var comm string
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		b, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "comm"))
		if err == nil {
			comm = strings.TrimSpace(string(b))
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if comm == "" {
		t.Skipf("could not read /proc/%d/comm; procfs unavailable", pid)
	}

	tun := model.Tunnel{PID: pid, ProcessName: comm}

	if err := reaper.TerminateTunnel("/proc", tun, 300*time.Millisecond); err != nil {
		t.Fatalf("expected SIGKILL escalation to succeed, got: %v", err)
	}

	_ = cmd.Wait()

	// After SIGKILL and reaping, the PID must no longer be signalable.
	if err := cmd.Process.Signal(syscall.Signal(0)); err == nil {
		t.Fatalf("expected process to be dead after SIGKILL escalation")
	}
}
