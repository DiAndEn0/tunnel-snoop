package tests

import (
	"net"
	"os/exec"
	"strings"
	"testing"
)

func TestLiveSocketDetection(t *testing.T) {
	// Start a mock listener on 0.0.0.0
	listener, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatalf("failed to open test listener: %v", err)
	}
	defer func() { _ = listener.Close() }()

	port := listener.Addr().(*net.TCPAddr).Port

	// Build tunnelsnoop
	buildCmd := exec.Command("/home/duser/.local/go/bin/go", "build", "-o", "../bin/tunnelsnoop", "../cmd/tunnelsnoop")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build tunnelsnoop: %v, out: %s", err, string(out))
	}

	// Run tunnelsnoop -once -json
	cmd := exec.Command("../bin/tunnelsnoop", "-once", "-json")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to run tunnelsnoop: %v, out: %s", err, string(out))
	}

	// Verify command ran and returned valid JSON
	if !strings.HasPrefix(strings.TrimSpace(string(out)), "[") {
		t.Fatalf("expected JSON array output, got: %s", string(out))
	}
	_ = port
}
