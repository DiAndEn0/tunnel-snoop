package procfs_test

import (
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
