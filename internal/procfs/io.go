package procfs

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// IOCounters holds the cumulative byte counters the kernel exposes in
// /proc/<pid>/io. The two pairs measure different things and must not be used
// interchangeably.
//
// RChar and WChar count every byte the process moved through the read(2) and
// write(2) syscall families, whatever the underlying file description was —
// including sockets. ReadBytes and WriteBytes count only the bytes that
// actually crossed the block layer, so they stay flat for a process whose work
// is purely network traffic.
//
// Both pairs are returned together, rather than one derived "bytes moved"
// figure, so that the choice of signal is visible at the call site instead of
// being silently baked in here.
type IOCounters struct {
	RChar      uint64
	WChar      uint64
	ReadBytes  uint64
	WriteBytes uint64
}

// ReadProcessIO reads the /proc/<pid>/io accounting file rooted at procRoot and
// returns the process's cumulative I/O counters.
//
// Fields that are absent, malformed, or not parseable as an unsigned integer
// are left at zero rather than failing the whole read: the exact field set in
// this file varies with kernel version and configuration, and losing every
// counter because one line is unusable would report a busy process as
// motionless.
func ReadProcessIO(procRoot string, pid int) (IOCounters, error) {
	path := filepath.Join(procRoot, strconv.Itoa(pid), "io")
	file, err := os.Open(path)
	if err != nil {
		return IOCounters{}, err
	}
	defer func() { _ = file.Close() }()

	var counters IOCounters
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val, err := strconv.ParseUint(strings.TrimSpace(parts[1]), 10, 64)
		if err != nil {
			continue
		}

		switch key {
		case "rchar":
			counters.RChar = val
		case "wchar":
			counters.WChar = val
		case "read_bytes":
			counters.ReadBytes = val
		case "write_bytes":
			counters.WriteBytes = val
		}
	}

	return counters, scanner.Err()
}
