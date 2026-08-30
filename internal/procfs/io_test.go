package procfs_test

import (
	"path/filepath"
	"testing"

	"github.com/DiAndEn0/tunnel-snoop/internal/procfs"
)

func TestReadProcessIO(t *testing.T) {
	procRoot := filepath.Join("testdata", "proc")
	readB, writeB, err := procfs.ReadProcessIO(procRoot, 101)
	if err != nil {
		t.Fatalf("unexpected error reading process io: %v", err)
	}

	if readB != 4096 || writeB != 8192 {
		t.Fatalf("expected 4096/8192, got %d/%d", readB, writeB)
	}
}

func TestReadProcessIOMissing(t *testing.T) {
	procRoot := filepath.Join("testdata", "proc")
	_, _, err := procfs.ReadProcessIO(procRoot, 999)
	if err == nil {
		t.Fatalf("expected error reading missing process io, got nil")
	}
}
