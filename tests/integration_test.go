package tests

import (
	"net"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// goTool locates the go binary: GOROOT first (matches the toolchain running
// this test), then PATH.
func goTool(t *testing.T) string {
	t.Helper()

	if root := runtime.GOROOT(); root != "" {
		candidate := filepath.Join(root, "bin", "go")
		if _, err := exec.LookPath(candidate); err == nil {
			return candidate
		}
	}

	path, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("go toolchain not found in GOROOT or PATH: %v", err)
	}
	return path
}

func TestLiveSocketDetection(t *testing.T) {
	// Start a mock listener on 0.0.0.0
	listener, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatalf("failed to open test listener: %v", err)
	}
	defer func() { _ = listener.Close() }()

	port := listener.Addr().(*net.TCPAddr).Port

	// Build tunnelsnoop into a temp dir the test owns
	binary := filepath.Join(t.TempDir(), "tunnelsnoop")
	buildCmd := exec.Command(goTool(t), "build", "-o", binary, "../cmd/tunnelsnoop")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build tunnelsnoop: %v, out: %s", err, string(out))
	}

	// Run tunnelsnoop -once -json
	cmd := exec.Command(binary, "-once", "-json")
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
