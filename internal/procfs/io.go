package procfs

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ReadProcessIO reads the /proc/<pid>/io accounting file rooted at procRoot
// and returns the cumulative bytes read from and written to storage by the
// process, as reported by the kernel's read_bytes and write_bytes fields.
func ReadProcessIO(procRoot string, pid int) (uint64, uint64, error) {
	path := filepath.Join(procRoot, strconv.Itoa(pid), "io")
	file, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer file.Close()

	var readBytes, writeBytes uint64
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
		case "read_bytes":
			readBytes = val
		case "write_bytes":
			writeBytes = val
		}
	}

	return readBytes, writeBytes, scanner.Err()
}
