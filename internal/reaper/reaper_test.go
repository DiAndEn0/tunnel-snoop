package reaper_test

import (
	"os/exec"
	"testing"
	"time"

	"github.com/DiAndEn0/tunnel-snoop/internal/model"
	"github.com/DiAndEn0/tunnel-snoop/internal/reaper"
)

func TestReaperTermination(t *testing.T) {
	cmd := exec.Command("sleep", "10")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start test process: %v", err)
	}

	tun := model.Tunnel{
		PID:         cmd.Process.Pid,
		ProcessName: "sleep",
	}

	err := reaper.TerminateTunnel("/proc", tun, 500*time.Millisecond)
	if err != nil {
		t.Fatalf("reaper failed: %v", err)
	}

	// Verify process is terminated
	_ = cmd.Wait()
}
