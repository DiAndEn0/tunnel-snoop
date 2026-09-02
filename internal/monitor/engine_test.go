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

// TestEngineReconciliation_ClientCountsAreProtocolScoped verifies that an
// established connection on tcp6 sharing the same numeric port as a tcp
// tunnel's listener does not get counted as an active client of that
// tunnel, since client counting must be scoped by (Protocol, LocalPort).
func TestEngineReconciliation_ClientCountsAreProtocolScoped(t *testing.T) {
	procRoot, netRoot := buildFixture(t, 0, 0)

	// Add an ESTABLISHED tcp6 connection on the same port number (5432 =
	// 0x1538) as the tcp tunnel's listener. It must not be counted.
	tcp6Contents := "  sl  local_address                         rem_address                            st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode\n" +
		"   0: 00000000000000000000000001000000:1538 00000000000000000000000000000000:0000 01 00000000:00000000 00:00000000 00000000  1000        0 55555 1 0000000000000000 100 0 0 10 0\n"
	if err := os.WriteFile(filepath.Join(netRoot, "net", "tcp6"), []byte(tcp6Contents), 0o644); err != nil {
		t.Fatalf("failed to write net/tcp6: %v", err)
	}

	eng := monitor.NewEngine(monitor.Config{
		ProcRoot:        procRoot,
		NetRoot:         netRoot,
		AllowedBinaries: []string{"kubectl"},
	})

	tunnels, err := eng.Reconcile(time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tunnels) != 1 {
		t.Fatalf("expected 1 tunnel, got %d", len(tunnels))
	}
	if tunnels[0].Protocol != model.ProtoIPv4 {
		t.Fatalf("expected tcp tunnel, got %q", tunnels[0].Protocol)
	}
	if tunnels[0].ActiveClients != 0 {
		t.Fatalf("expected tcp6 connection on the same port not to be counted, got %d active clients", tunnels[0].ActiveClients)
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

// TestEngineReconciliation_MultipleListenersUnderSinglePID tests that a single
// process (e.g. ssh -L 8080:... -L 9090:...) holding multiple listening sockets
// reports ALL tunnels independently without deduplicating or dropping listeners.
func TestEngineReconciliation_MultipleListenersUnderSinglePID(t *testing.T) {
	root := t.TempDir()
	procRoot := filepath.Join(root, "proc")
	netRoot := filepath.Join(root, "net_root")
	pidDir := filepath.Join(procRoot, "101")

	if err := os.MkdirAll(filepath.Join(pidDir, "fd"), 0o755); err != nil {
		t.Fatalf("failed to build proc root: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(netRoot, "net"), 0o755); err != nil {
		t.Fatalf("failed to build net root: %v", err)
	}

	if err := os.WriteFile(filepath.Join(pidDir, "comm"), []byte("ssh\n"), 0o644); err != nil {
		t.Fatalf("failed to write comm: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pidDir, "cmdline"), []byte("ssh\x00-L\x008080:localhost:8080\x00-L\x009090:localhost:9090\x00"), 0o644); err != nil {
		t.Fatalf("failed to write cmdline: %v", err)
	}
	writeIO(t, pidDir, 1000, 2000)

	// Two listening sockets under the same PID:
	// Inode 1001: 127.0.0.1:8080 (0100007F:1F90)
	// Inode 1002: 0.0.0.0:9090   (00000000:2382)
	if err := os.Symlink("socket:[1001]", filepath.Join(pidDir, "fd", "3")); err != nil {
		t.Fatalf("failed to symlink fd 3: %v", err)
	}
	if err := os.Symlink("socket:[1002]", filepath.Join(pidDir, "fd", "4")); err != nil {
		t.Fatalf("failed to symlink fd 4: %v", err)
	}

	tcpContents := "  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode\n" +
		"   0: 0100007F:1F90 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 1001 1 0000000000000000 100 0 0 10 0\n" +
		"   1: 00000000:2382 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 1002 1 0000000000000000 100 0 0 10 0\n"
	if err := os.WriteFile(filepath.Join(netRoot, "net", "tcp"), []byte(tcpContents), 0o644); err != nil {
		t.Fatalf("failed to write net/tcp: %v", err)
	}

	eng := monitor.NewEngine(monitor.Config{
		ProcRoot:        procRoot,
		NetRoot:         netRoot,
		AllowedBinaries: []string{"ssh"},
	})

	t0 := time.Now()
	tunnels1, err := eng.Reconcile(t0)
	if err != nil {
		t.Fatalf("unexpected error on pass 1: %v", err)
	}
	if len(tunnels1) != 2 {
		t.Fatalf("pass 1: expected 2 tunnels, got %d", len(tunnels1))
	}

	// Verify both sockets exist and are distinct
	found8080, found9090 := false, false
	for _, tun := range tunnels1 {
		if tun.LocalPort == 8080 && tun.SocketInode == 1001 && tun.LocalAddress == "127.0.0.1" && !tun.IsWildcard {
			found8080 = true
		}
		if tun.LocalPort == 9090 && tun.SocketInode == 1002 && tun.LocalAddress == "0.0.0.0" && tun.IsWildcard {
			found9090 = true
		}
	}
	if !found8080 || !found9090 {
		t.Fatalf("pass 1: missing expected tunnels, found 8080=%v, 9090(wildcard)=%v", found8080, found9090)
	}

	// Pass 2: state must be preserved across iterations without clobbering
	t1 := t0.Add(5 * time.Second)
	tunnels2, err := eng.Reconcile(t1)
	if err != nil {
		t.Fatalf("unexpected error on pass 2: %v", err)
	}
	if len(tunnels2) != 2 {
		t.Fatalf("pass 2: expected 2 tunnels, got %d", len(tunnels2))
	}
	found8080, found9090 = false, false
	for _, tun := range tunnels2 {
		if tun.LocalPort == 8080 && tun.SocketInode == 1001 && tun.LocalAddress == "127.0.0.1" && !tun.IsWildcard {
			found8080 = true
		}
		if tun.LocalPort == 9090 && tun.SocketInode == 1002 && tun.LocalAddress == "0.0.0.0" && tun.IsWildcard {
			found9090 = true
		}
	}
	if !found8080 || !found9090 {
		t.Fatalf("pass 2: missing expected tunnels, found 8080=%v, 9090(wildcard)=%v", found8080, found9090)
	}

	// Pass 3: Close one listener (remove inode 1001 from tcp and fd 3)
	if err := os.Remove(filepath.Join(pidDir, "fd", "3")); err != nil {
		t.Fatalf("failed to remove fd 3: %v", err)
	}
	tcpContentsOnly9090 := "  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode\n" +
		"   1: 00000000:2382 00000000:0000 0A 00000000:00000000 00:00000000 00000000  1000        0 1002 1 0000000000000000 100 0 0 10 0\n"
	if err := os.WriteFile(filepath.Join(netRoot, "net", "tcp"), []byte(tcpContentsOnly9090), 0o644); err != nil {
		t.Fatalf("failed to write net/tcp: %v", err)
	}

	t2 := t1.Add(5 * time.Second)
	tunnels3, err := eng.Reconcile(t2)
	if err != nil {
		t.Fatalf("unexpected error on pass 3: %v", err)
	}
	if len(tunnels3) != 1 {
		t.Fatalf("pass 3: expected 1 tunnel after closing one listener, got %d", len(tunnels3))
	}
	if tunnels3[0].LocalPort != 9090 || tunnels3[0].SocketInode != 1002 {
		t.Fatalf("pass 3: expected remaining tunnel on port 9090 inode 1002, got port %d inode %d", tunnels3[0].LocalPort, tunnels3[0].SocketInode)
	}
}
