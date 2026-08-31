package monitor_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DiAndEn0/tunnel-snoop/internal/monitor"
)

// emptyNetRoot returns a directory holding a header-only net/tcp table, so
// socket parsing succeeds with zero entries and does not mask errors raised
// later in the reconcile pass.
func emptyNetRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	netDir := filepath.Join(root, "net")
	if err := os.MkdirAll(netDir, 0o755); err != nil {
		t.Fatalf("failed to create net dir: %v", err)
	}
	header := "  sl  local_address rem_address   st tx_queue rx_queue tr tm->when retrnsmt   uid  timeout inode\n"
	if err := os.WriteFile(filepath.Join(netDir, "tcp"), []byte(header), 0o644); err != nil {
		t.Fatalf("failed to write net/tcp: %v", err)
	}
	return root
}

// Reconcile must surface a failure to enumerate the process table rather than
// silently reporting zero tunnels, which would look identical to "nothing is
// listening" and mask a broken procfs mount.
func TestReconcileReturnsErrorWhenProcRootUnreadable(t *testing.T) {
	e := monitor.NewEngine(monitor.Config{
		ProcRoot:        filepath.Join(t.TempDir(), "does-not-exist"),
		NetRoot:         emptyNetRoot(t),
		AllowedBinaries: []string{"ssh"},
	})

	tunnels, err := e.Reconcile(time.Now())
	if err == nil {
		t.Fatalf("expected error for unreadable ProcRoot, got nil (tunnels: %v)", tunnels)
	}
	if tunnels != nil {
		t.Fatalf("expected nil tunnels alongside error, got %v", tunnels)
	}
}

// An unreadable socket table is likewise an error rather than an empty view.
func TestReconcileReturnsErrorWhenNetRootUnreadable(t *testing.T) {
	e := monitor.NewEngine(monitor.Config{
		ProcRoot:        t.TempDir(),
		NetRoot:         filepath.Join(t.TempDir(), "does-not-exist"),
		AllowedBinaries: []string{"ssh"},
	})

	tunnels, err := e.Reconcile(time.Now())
	if err == nil {
		t.Fatalf("expected error for unreadable NetRoot, got nil (tunnels: %v)", tunnels)
	}
	if tunnels != nil {
		t.Fatalf("expected nil tunnels alongside error, got %v", tunnels)
	}
}
