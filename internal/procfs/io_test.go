package procfs_test

import (
	"path/filepath"
	"testing"

	"github.com/DiAndEn0/tunnel-snoop/internal/procfs"
)

func TestReadProcessIO(t *testing.T) {
	procRoot := filepath.Join("testdata", "proc")
	io, err := procfs.ReadProcessIO(procRoot, 101)
	if err != nil {
		t.Fatalf("unexpected error reading process io: %v", err)
	}

	if io.RChar != 12345 || io.WChar != 67890 {
		t.Fatalf("expected rchar/wchar 12345/67890, got %d/%d", io.RChar, io.WChar)
	}

	// The block-device counters are still parsed even though they are no
	// longer the activity signal, so a caller that genuinely wants disk I/O
	// can ask for it explicitly.
	if io.ReadBytes != 4096 || io.WriteBytes != 8192 {
		t.Fatalf("expected read_bytes/write_bytes 4096/8192, got %d/%d", io.ReadBytes, io.WriteBytes)
	}
}

// TestReadProcessIOPartial covers a truncated and partly malformed accounting
// file: a value that does not parse, a line without the "key: value"
// separator, and fields that are simply absent. None of these may abort the
// read, because /proc/<pid>/io content varies with kernel configuration and a
// single unusable line must not cost us the fields that did parse.
func TestReadProcessIOPartial(t *testing.T) {
	procRoot := filepath.Join("testdata", "proc")
	io, err := procfs.ReadProcessIO(procRoot, 102)
	if err != nil {
		t.Fatalf("unexpected error reading partial process io: %v", err)
	}

	if io.RChar != 555 {
		t.Fatalf("expected rchar 555, got %d", io.RChar)
	}
	if io.WChar != 0 {
		t.Fatalf("expected unparseable wchar to yield 0, got %d", io.WChar)
	}
	if io.ReadBytes != 0 || io.WriteBytes != 0 {
		t.Fatalf("expected absent block counters to yield 0/0, got %d/%d", io.ReadBytes, io.WriteBytes)
	}
}

func TestReadProcessIOMissing(t *testing.T) {
	procRoot := filepath.Join("testdata", "proc")
	io, err := procfs.ReadProcessIO(procRoot, 999)
	if err == nil {
		t.Fatalf("expected error reading missing process io, got nil")
	}
	if io != (procfs.IOCounters{}) {
		t.Fatalf("expected zero counters alongside the error, got %+v", io)
	}
}
