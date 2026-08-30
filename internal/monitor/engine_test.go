package monitor_test

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/DiAndEn0/tunnel-snoop/internal/model"
	"github.com/DiAndEn0/tunnel-snoop/internal/monitor"
)

func TestEngineReconciliation(t *testing.T) {
	eng := monitor.NewEngine(monitor.Config{
		ProcRoot:        "../procfs/testdata/proc",
		NetRoot:         "../procfs/testdata",
		AllowedBinaries: []string{"kubectl"},
	})

	t0 := time.Now()
	tunnels, err := eng.Reconcile(t0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tunnels) != 1 {
		t.Fatalf("expected 1 tunnel, got %d", len(tunnels))
	}

	if tunnels[0].ActiveClients != 1 {
		t.Fatalf("expected 1 active client, got %d", tunnels[0].ActiveClients)
	}

	if tunnels[0].Protocol != model.ProtoIPv4 {
		t.Fatalf("expected tcp protocol, got %q", tunnels[0].Protocol)
	}
}

func TestEngineReconciliation_UnknownBinaryYieldsNoTunnels(t *testing.T) {
	eng := monitor.NewEngine(monitor.Config{
		ProcRoot:        "../procfs/testdata/proc",
		NetRoot:         "../procfs/testdata",
		AllowedBinaries: []string{"ssh"},
	})

	tunnels, err := eng.Reconcile(time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tunnels) != 0 {
		t.Fatalf("expected 0 tunnels, got %d", len(tunnels))
	}
}

// buildFixture creates a self-contained proc+net root with a single
// "kubectl" tunnel process (pid 101) listening on 127.0.0.1:5432 (inode
// 45678), with no established client connections, so tests can control
// ActiveClients and I/O byte counters precisely.
func buildFixture(t *testing.T, readBytes, writeBytes uint64) (procRoot, netRoot string) {
	t.Helper()
	root := t.TempDir()

	procRoot = filepath.Join(root, "proc")
	netRoot = filepath.Join(root, "net_root")
	pidDir := filepath.Join(procRoot, "101")

	if err := os.MkdirAll(filepath.Join(pidDir, "fd"), 0o755); err != nil {
		t.Fatalf("failed to build proc root: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(netRoot, "net"), 0o755); err != nil {
		t.Fatalf("failed to build net root: %v", err)
	}

	if err := os.WriteFile(filepath.Join(pidDir, "comm"), []byte("kubectl\n"), 0o644); err != nil {
		t.Fatalf("failed to write comm: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pidDir, "cmdline"), []byte("kubectl\x00port-forward\x00svc/db\x005432:5432\x00"), 0o644); err != nil {
		t.Fatalf("failed to write cmdline: %v", err)
	}
	writeIO(t, pidDir, readBytes, writeBytes)
	if err := os.Symlink("socket:[45678]", filepath.Join(pidDir, "fd", "3")); err != nil {
		t.Fatalf("failed to symlink fd: %v", err)
	}

	tcpContents := "  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode\n" +
		"   0: 0100007F:1538 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 45678 1 0000000000000000 100 0 0 10 0\n"
	if err := os.WriteFile(filepath.Join(netRoot, "net", "tcp"), []byte(tcpContents), 0o644); err != nil {
		t.Fatalf("failed to write net/tcp: %v", err)
	}

	return procRoot, netRoot
}

func writeIO(t *testing.T, pidDir string, readBytes, writeBytes uint64) {
	t.Helper()
	contents := "read_bytes: " + strconv.FormatUint(readBytes, 10) + "\nwrite_bytes: " + strconv.FormatUint(writeBytes, 10) + "\n"
	if err := os.WriteFile(filepath.Join(pidDir, "io"), []byte(contents), 0o644); err != nil {
		t.Fatalf("failed to write io: %v", err)
	}
}

func TestEngineReconciliation_HoldsLastActiveWhenIdle(t *testing.T) {
	procRoot, netRoot := buildFixture(t, 4096, 8192)

	eng := monitor.NewEngine(monitor.Config{
		ProcRoot:        procRoot,
		NetRoot:         netRoot,
		AllowedBinaries: []string{"kubectl"},
	})

	t0 := time.Now()
	first, err := eng.Reconcile(t0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(first) != 1 {
		t.Fatalf("expected 1 tunnel, got %d", len(first))
	}
	if first[0].ActiveClients != 0 {
		t.Fatalf("expected 0 active clients, got %d", first[0].ActiveClients)
	}
	if first[0].BytesRead != 4096 || first[0].BytesWritten != 8192 {
		t.Fatalf("unexpected io bytes: read=%d write=%d", first[0].BytesRead, first[0].BytesWritten)
	}
	if !first[0].LastActive.Equal(t0) {
		t.Fatalf("expected LastActive to equal t0 on first sighting")
	}
	if first[0].IdleDuration != 0 {
		t.Fatalf("expected 0 idle duration on first sighting, got %v", first[0].IdleDuration)
	}

	// Second reconciliation with unchanged IO and no clients, at a later
	// timestamp: LastActive must not advance, but IdleDuration must reflect
	// the elapsed gap.
	t1 := t0.Add(5 * time.Second)
	second, err := eng.Reconcile(t1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(second) != 1 {
		t.Fatalf("expected 1 tunnel on second reconcile, got %d", len(second))
	}
	if !second[0].LastActive.Equal(t0) {
		t.Fatalf("expected LastActive to remain at t0 when idle, got %v", second[0].LastActive)
	}
	if second[0].IdleDuration != 5*time.Second {
		t.Fatalf("expected idle duration of 5s, got %v", second[0].IdleDuration)
	}

	// Third reconciliation with a change in I/O bytes (activity resumes):
	// LastActive must advance and IdleDuration must reset to 0.
	writeIO(t, filepath.Join(procRoot, "101"), 4096, 16384)
	t2 := t1.Add(5 * time.Second)
	third, err := eng.Reconcile(t2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !third[0].LastActive.Equal(t2) {
		t.Fatalf("expected LastActive to advance to t2 on IO activity, got %v", third[0].LastActive)
	}
	if third[0].IdleDuration != 0 {
		t.Fatalf("expected idle duration to reset to 0 after activity, got %v", third[0].IdleDuration)
	}
	if third[0].BytesWritten != 16384 {
		t.Fatalf("expected updated write byte count of 16384, got %d", third[0].BytesWritten)
	}
}

func TestEngineReconciliation_EvictsTerminatedProcess(t *testing.T) {
	procRoot, netRoot := buildFixture(t, 100, 200)

	eng := monitor.NewEngine(monitor.Config{
		ProcRoot:        procRoot,
		NetRoot:         netRoot,
		AllowedBinaries: []string{"kubectl"},
	})

	t0 := time.Now()
	present, err := eng.Reconcile(t0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(present) != 1 {
		t.Fatalf("expected 1 tunnel before termination, got %d", len(present))
	}

	// Terminate the process: remove its proc directory entirely, then
	// reconcile again on the SAME engine to verify the cached tunnel is
	// evicted from internal state.
	if err := os.RemoveAll(filepath.Join(procRoot, "101")); err != nil {
		t.Fatalf("failed to remove pid dir: %v", err)
	}

	t1 := t0.Add(5 * time.Second)
	gone, err := eng.Reconcile(t1)
	if err != nil {
		t.Fatalf("unexpected error after termination: %v", err)
	}
	if len(gone) != 0 {
		t.Fatalf("expected 0 tunnels after process termination, got %d", len(gone))
	}
}
