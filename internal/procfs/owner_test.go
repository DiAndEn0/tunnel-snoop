package procfs_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/DiAndEn0/tunnel-snoop/internal/model"
	"github.com/DiAndEn0/tunnel-snoop/internal/procfs"
)

// listenSocket is the fixture socket that testdata/proc/101 holds an fd for.
func listenSocket() []model.SocketEntry {
	return []model.SocketEntry{
		{
			Protocol:  model.ProtoIPv4,
			LocalIP:   "127.0.0.1",
			LocalPort: 5432,
			State:     model.StateListen,
			Inode:     45678,
		},
	}
}

// Ownership filtering is what keeps a privileged run from discovering, and
// therefore reaping, other users' tunnels. Unprivileged the fd traversal
// already fails with EACCES on foreign processes, so the filter has to be
// exercised directly rather than inferred from a normal scan.
func TestFindTunnelsSkipsProcessesOwnedByAnotherUser(t *testing.T) {
	procRoot := filepath.Join("testdata", "proc")

	// A UID that cannot own the fixtures: one past the current user's.
	foreign := os.Getuid() + 1

	tunnels, err := procfs.FindTunnels(procRoot, listenSocket(), []string{"kubectl"}, foreign)
	if err != nil {
		t.Fatalf("FindTunnels returned an error: %v", err)
	}
	if len(tunnels) != 0 {
		t.Fatalf("expected no tunnels for a foreign UID, got %d: %+v", len(tunnels), tunnels)
	}
}

func TestFindTunnelsIncludesProcessesOwnedByCaller(t *testing.T) {
	procRoot := filepath.Join("testdata", "proc")

	tunnels, err := procfs.FindTunnels(procRoot, listenSocket(), []string{"kubectl"}, os.Getuid())
	if err != nil {
		t.Fatalf("FindTunnels returned an error: %v", err)
	}
	if len(tunnels) != 1 {
		t.Fatalf("expected the caller-owned fixture tunnel, got %d: %+v", len(tunnels), tunnels)
	}
	if tunnels[0].PID != 101 {
		t.Fatalf("expected PID 101, got %d", tunnels[0].PID)
	}
}
