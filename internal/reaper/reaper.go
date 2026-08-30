package reaper

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/DiAndEn0/tunnel-snoop/internal/model"
)

func TerminateTunnel(procRoot string, tunnel model.Tunnel, gracePeriod time.Duration) error {
	// 1. Re-verify process identity before sending signal
	commPath := filepath.Join(procRoot, fmt.Sprintf("%d", tunnel.PID), "comm")
	commBytes, err := os.ReadFile(commPath)
	if err != nil {
		return fmt.Errorf("process %d already exited or unreadable: %w", tunnel.PID, err)
	}

	comm := strings.TrimSpace(string(commBytes))
	if !strings.EqualFold(comm, tunnel.ProcessName) {
		return fmt.Errorf("PID %d reused: expected %s, found %s; aborting kill", tunnel.PID, tunnel.ProcessName, comm)
	}

	// 2. Re-verify the tunnel's listening socket is still owned by this PID.
	// This guards against the window between discovery and termination where
	// the socket was closed (or the PID recycled for an unrelated process
	// sharing the same binary name) despite comm still matching.
	if tunnel.SocketInode > 0 {
		if err := verifySocketInode(procRoot, tunnel.PID, tunnel.SocketInode); err != nil {
			return err
		}
	}

	// 3. Send SIGTERM
	proc, err := os.FindProcess(tunnel.PID)
	if err != nil {
		return err
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM to PID %d: %w", tunnel.PID, err)
	}

	// 4. Wait up to gracePeriod for process exit
	deadline := time.Now().Add(gracePeriod)
	for time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
		// Check if process still exists
		if err := proc.Signal(syscall.Signal(0)); err != nil {
			return nil // Process exited
		}
	}

	// 5. Escalate to SIGKILL if still alive
	if err := proc.Signal(syscall.SIGKILL); err != nil {
		return fmt.Errorf("failed to escalate to SIGKILL for PID %d: %w", tunnel.PID, err)
	}

	return nil
}

// verifySocketInode confirms that PID still holds an open file descriptor
// pointing to socket:[inode] under procRoot. It returns a descriptive error
// if the fd directory cannot be read, or if no matching fd is found (e.g.
// because the socket was closed or the PID was recycled for a different
// process).
func verifySocketInode(procRoot string, pid int, inode uint64) error {
	fdDir := filepath.Join(procRoot, fmt.Sprintf("%d", pid), "fd")
	fds, err := os.ReadDir(fdDir)
	if err != nil {
		return fmt.Errorf("cannot read fd directory for PID %d: %w", pid, err)
	}

	want := fmt.Sprintf("socket:[%d]", inode)
	for _, fd := range fds {
		link, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
		if err != nil {
			continue
		}
		if link == want {
			return nil
		}
	}

	return fmt.Errorf("PID %d no longer holds socket inode %d (socket closed or PID recycled); aborting kill", pid, inode)
}
