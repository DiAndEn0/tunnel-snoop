package procfs_test

import (
	"path/filepath"
	"testing"

	"github.com/DiAndEn0/tunnel-snoop/internal/model"
	"github.com/DiAndEn0/tunnel-snoop/internal/procfs"
)

func TestParseSockets(t *testing.T) {
	testDir := filepath.Join("testdata")
	sockets, err := procfs.ParseSockets(testDir)
	if err != nil {
		t.Fatalf("unexpected error parsing sockets: %v", err)
	}

	if len(sockets) != 5 {
		t.Fatalf("expected 5 sockets, got %d", len(sockets))
	}

	// Verify loopback IPv4 listen socket (0100007F:1538 -> 127.0.0.1:5432)
	s0 := sockets[0]
	if s0.LocalIP != "127.0.0.1" || s0.LocalPort != 5432 || s0.State != model.StateListen || s0.Inode != 45678 {
		t.Fatalf("s0 mismatch: %+v", s0)
	}

	// Verify 0.0.0.0 IPv4 listen socket (00000000:1538 -> 0.0.0.0:5432)
	s1 := sockets[1]
	if s1.LocalIP != "0.0.0.0" || s1.LocalPort != 5432 || s1.State != model.StateListen || s1.Inode != 45679 {
		t.Fatalf("s1 mismatch: %+v", s1)
	}

	// Verify established socket
	s2 := sockets[2]
	if s2.State != model.StateEstablished || s2.LocalPort != 5432 || s2.RemotePort != 54321 {
		t.Fatalf("s2 mismatch: %+v", s2)
	}

	// Verify IPv6 loopback (::1:8080)
	s3 := sockets[3]
	if s3.LocalIP != "::1" || s3.LocalPort != 8080 || s3.State != model.StateListen {
		t.Fatalf("s3 mismatch: %+v", s3)
	}

	// Verify IPv6 wildcard (:::8080)
	s4 := sockets[4]
	if s4.LocalIP != "::" || s4.LocalPort != 8080 || s4.State != model.StateListen {
		t.Fatalf("s4 mismatch: %+v", s4)
	}
}
