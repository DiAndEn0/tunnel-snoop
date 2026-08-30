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
	}

	// 4. Escalate to SIGKILL if still alive
	if err := proc.Signal(syscall.SIGKILL); err != nil {
		return fmt.Errorf("failed to escalate to SIGKILL for PID %d: %w", tunnel.PID, err)
	}

	return nil
}
