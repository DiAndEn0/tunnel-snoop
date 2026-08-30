package procfs_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/DiAndEn0/tunnel-snoop/internal/model"
	"github.com/DiAndEn0/tunnel-snoop/internal/procfs"
)

func TestFindTunnels(t *testing.T) {
	procRoot := filepath.Join("testdata", "proc")
	sockets := []model.SocketEntry{
		{
			Protocol:  model.ProtoIPv4,
			LocalIP:   "127.0.0.1",
			LocalPort: 5432,
			State:     model.StateListen,
			Inode:     45678,
		},
	}

	allowed := []string{"kubectl", "ssh", "cloudflared", "ngrok"}
	tunnels, err := procfs.FindTunnels(procRoot, sockets, allowed)
	if err != nil {
		t.Fatalf("unexpected error finding tunnels: %v", err)
	}

	if len(tunnels) != 1 {
		t.Fatalf("expected 1 tunnel, got %d", len(tunnels))
	}

	tun := tunnels[0]
	if tun.PID != 101 || tun.ProcessName != "kubectl" || tun.LocalPort != 5432 {
		t.Fatalf("tunnel mismatch: %+v", tun)
	}
}

// TestFindTunnels_DedupesDuplicateFdsToSameInode verifies that when a
// process holds multiple fds pointing at the same listening socket inode
// (e.g. a dup'd listener fd), FindTunnels produces exactly one Tunnel entry
// rather than one per matching fd.
func TestFindTunnels_DedupesDuplicateFdsToSameInode(t *testing.T) {
	procRoot := t.TempDir()
	pidDir := filepath.Join(procRoot, "202")
	fdDir := filepath.Join(pidDir, "fd")
	if err := os.MkdirAll(fdDir, 0o755); err != nil {
		t.Fatalf("failed to build proc root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pidDir, "comm"), []byte("kubectl\n"), 0o644); err != nil {
		t.Fatalf("failed to write comm: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pidDir, "cmdline"), []byte("kubectl\x00port-forward\x00svc/db\x005432:5432\x00"), 0o644); err != nil {
		t.Fatalf("failed to write cmdline: %v", err)
	}
	// Three fds, but only two distinct inodes: 3 and 5 both point at the
	// same listening socket, simulating a dup'd fd.
	if err := os.Symlink("socket:[45678]", filepath.Join(fdDir, "3")); err != nil {
		t.Fatalf("failed to symlink fd 3: %v", err)
	}
	if err := os.Symlink("socket:[45678]", filepath.Join(fdDir, "5")); err != nil {
		t.Fatalf("failed to symlink fd 5: %v", err)
	}

	sockets := []model.SocketEntry{
		{
			Protocol:  model.ProtoIPv4,
			LocalIP:   "127.0.0.1",
			LocalPort: 5432,
			State:     model.StateListen,
			Inode:     45678,
		},
	}

	tunnels, err := procfs.FindTunnels(procRoot, sockets, []string{"kubectl"})
	if err != nil {
		t.Fatalf("unexpected error finding tunnels: %v", err)
	}

	if len(tunnels) != 1 {
		t.Fatalf("expected exactly 1 deduplicated tunnel, got %d: %+v", len(tunnels), tunnels)
	}
}
