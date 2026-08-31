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
	if err := verifyIdentity(procRoot, tunnel); err != nil {
		return err
	}

	// 2. Send SIGTERM
	proc, err := os.FindProcess(tunnel.PID)
	if err != nil {
		return err
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send SIGTERM to PID %d: %w", tunnel.PID, err)
	}

	// 3. Wait up to gracePeriod for process exit
	deadline := time.Now().Add(gracePeriod)
	for time.Now().Before(deadline) {
		time.Sleep(50 * time.Millisecond)
		// Check if process still exists
		if err := proc.Signal(syscall.Signal(0)); err != nil {
			return nil // Process exited
		}
		// A process that exited but has not been reaped by its parent stays
		// signalable as a zombie, so signal 0 alone would run the loop to
		// exhaustion and then escalate against a process that is already dead.
		if hasExited(procRoot, tunnel.PID) {
			return nil
		}
	}

	// 4. Re-verify identity before escalating. The target is not our child, so
	// nothing pins its PID: it may have exited during the grace period and had
	// the PID recycled, in which case signal 0 above succeeded against an
	// unrelated process. Repeating the checks keeps SIGKILL under the same
	// guarantees as SIGTERM rather than trusting a decision made seconds ago.
	if err := verifyIdentity(procRoot, tunnel); err != nil {
		return fmt.Errorf("aborting SIGKILL escalation for PID %d: %w", tunnel.PID, err)
	}

	// 5. Escalate to SIGKILL if still alive
	if err := proc.Signal(syscall.SIGKILL); err != nil {
		return fmt.Errorf("failed to escalate to SIGKILL for PID %d: %w", tunnel.PID, err)
	}

	return nil
}

// hasExited reports whether pid has terminated but not yet been reaped by its
// parent. Such a process remains signalable, so signal 0 cannot distinguish it
// from a live one. It reports false when the state cannot be determined, which
// keeps the caller on its existing signal-based path rather than treating an
// unreadable procfs as a death.
func hasExited(procRoot string, pid int) bool {
	stat, err := os.ReadFile(filepath.Join(procRoot, fmt.Sprintf("%d", pid), "stat"))
	if err != nil {
		return false
	}

	// Field 2 is the executable name in parentheses and may itself contain
	// spaces or parentheses, so the state character is located relative to the
	// final ')' rather than by splitting the whole line.
	close := strings.LastIndex(string(stat), ")")
	if close < 0 {
		return false
	}

	fields := strings.Fields(string(stat)[close+1:])
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
// tunnel, by comparing /proc/<pid>/comm against the discovered binary name and
// confirming the listening socket is still held. It is checked before every
// signal, since the process is not a child of this program and its PID may be
// recycled at any point after discovery.
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

	// The socket check guards the narrower case where the PID was recycled for
	// an unrelated process sharing the same binary name, or where the tunnel
	// closed its listener but the process itself is still running.
	if tunnel.SocketInode > 0 {
		return verifySocketInode(procRoot, tunnel.PID, tunnel.SocketInode)
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
