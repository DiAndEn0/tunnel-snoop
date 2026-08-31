package procfs_test

import (
	"os"
	"path/filepath"
	"strings"
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

// A host with IPv6 disabled has no net/tcp6; the IPv4 table alone is still a
// valid socket view and must not be reported as a failure.
func TestParseSocketsToleratesMissingIPv6Table(t *testing.T) {
	root := t.TempDir()
	netDir := filepath.Join(root, "net")
	if err := os.MkdirAll(netDir, 0o755); err != nil {
		t.Fatalf("failed to create net dir: %v", err)
	}

	src, err := os.ReadFile(filepath.Join("testdata", "net", "tcp"))
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}
	if err := os.WriteFile(filepath.Join(netDir, "tcp"), src, 0o644); err != nil {
		t.Fatalf("failed to write tcp fixture: %v", err)
	}

	sockets, err := procfs.ParseSockets(root)
	if err != nil {
		t.Fatalf("expected missing tcp6 to be tolerated, got: %v", err)
	}
	if len(sockets) == 0 {
		t.Fatalf("expected IPv4 sockets to be parsed")
	}
	for _, s := range sockets {
		if s.Protocol != model.ProtoIPv4 {
			t.Fatalf("expected only IPv4 entries, got %+v", s)
		}
	}
}

// When neither table can be read the result is empty for want of data, not
// because nothing is listening. Returning nil there would make a broken procfs
// mount indistinguishable from an idle host.
func TestParseSocketsErrorsWhenNoTableReadable(t *testing.T) {
	sockets, err := procfs.ParseSockets(t.TempDir())
	if err == nil {
		t.Fatalf("expected error when no socket table is readable, got nil (sockets: %v)", sockets)
	}
	if sockets != nil {
		t.Fatalf("expected nil sockets alongside error, got %v", sockets)
	}
	if !strings.Contains(err.Error(), "no readable socket table") {
		t.Fatalf("unexpected error: %v", err)
	}
}
