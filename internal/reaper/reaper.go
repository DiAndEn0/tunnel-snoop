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
	if err := verifyIdentity(procRoot, tunnel); err != nil {
		return err
	}

	proc, err := os.FindProcess(tunnel.PID)
	if err != nil {
		return err
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM to PID %d: %w", tunnel.PID, err)
	}

	deadline := time.Now().Add(gracePeriod)
	for time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
		if err := proc.Signal(syscall.Signal(0)); err != nil {
			return nil
		}
		// A process that exited but has not been reaped by its parent stays
		// signalable as a zombie, so signal 0 alone would run the loop to
		// exhaustion and then escalate against a process that is already dead.
		if hasExited(procRoot, tunnel.PID) {
			return nil
		}
	}

	// Re-verify identity before escalating to SIGKILL in case PID was recycled
	// during the grace period.
	if err := verifyIdentity(procRoot, tunnel); err != nil {
		return fmt.Errorf("aborting SIGKILL escalation for PID %d: %w", tunnel.PID, err)
	}

	if err := proc.Signal(syscall.SIGKILL); err != nil {
		return fmt.Errorf("failed to escalate to SIGKILL for PID %d: %w", tunnel.PID, err)
	}

	return nil
}

// hasExited reports whether pid has terminated but not yet been reaped by its
// parent. Such a process remains signalable, so signal 0 cannot distinguish it
// from a live one.
func hasExited(procRoot string, pid int) bool {
	stat, err := os.ReadFile(filepath.Join(procRoot, fmt.Sprintf("%d", pid), "stat"))
	if err != nil {
		return false
	}

	closeIdx := strings.LastIndex(string(stat), ")")
	if closeIdx < 0 {
		return false
	}

	fields := strings.Fields(string(stat)[closeIdx+1:])
	if len(fields) == 0 {
		return false
	}

	// Z is a reaped-pending zombie; X and x are the transient dead states.
	switch fields[0] {
	case "Z", "X", "x":
		return true
	default:
		return false
	}
}

// verifyIdentity confirms that pid still refers to the process recorded in
// tunnel by checking binary name and listening socket ownership.
func verifyIdentity(procRoot string, tunnel model.Tunnel) error {
	commPath := filepath.Join(procRoot, fmt.Sprintf("%d", tunnel.PID), "comm")
	commBytes, err := os.ReadFile(commPath)
	if err != nil {
		return fmt.Errorf("process %d already exited or unreadable: %w", tunnel.PID, err)
	}

	comm := strings.TrimSpace(string(commBytes))
	if !strings.EqualFold(comm, tunnel.ProcessName) {
		return fmt.Errorf("PID %d reused: expected %s, found %s; aborting kill", tunnel.PID, tunnel.ProcessName, comm)
	}

	if tunnel.SocketInode > 0 {
		return verifySocketInode(procRoot, tunnel.PID, tunnel.SocketInode)
	}

	return nil
}

// verifySocketInode confirms that PID still holds an open file descriptor
// pointing to socket:[inode] under procRoot.
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
