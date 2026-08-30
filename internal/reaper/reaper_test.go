package reaper_test

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
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

// socketInode returns the procfs inode backing fd's underlying socket, by
// reading the /proc/self/fd/<n> symlink (which the kernel renders as
// "socket:[<inode>]").
func socketInode(t *testing.T, fd uintptr) uint64 {
	t.Helper()
	link, err := os.Readlink(fmt.Sprintf("/proc/self/fd/%d", fd))
	if err != nil {
		t.Fatalf("failed to read fd symlink: %v", err)
	}
	if !strings.HasPrefix(link, "socket:[") || !strings.HasSuffix(link, "]") {
		t.Fatalf("unexpected fd link format: %s", link)
	}
	inode, err := strconv.ParseUint(link[len("socket:["):len(link)-1], 10, 64)
	if err != nil {
		t.Fatalf("failed to parse inode from %s: %v", link, err)
	}
	return inode
}

func TestReaperTermination_SucceedsWhenSocketInodeStillHeld(t *testing.T) {
	lst, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to open listener: %v", err)
	}

	// Dup the listener's fd into an *os.File we can hand to the child
	// process via ExtraFiles; this transfers ownership away from lst.
	tcpLst, ok := lst.(*net.TCPListener)
	if !ok {
		t.Fatalf("expected *net.TCPListener")
	}
	f, err := tcpLst.File()
	if err != nil {
		t.Fatalf("failed to extract listener fd: %v", err)
	}
	inode := socketInode(t, f.Fd())
	_ = lst.Close()

	cmd := exec.Command("sleep", "10")
	cmd.ExtraFiles = []*os.File{f}
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start test process: %v", err)
	}
	_ = f.Close() // parent's copy no longer needed; child retains its own

	tun := model.Tunnel{
		PID:         cmd.Process.Pid,
		ProcessName: "sleep",
		SocketInode: inode,
	}

	if err := reaper.TerminateTunnel("/proc", tun, 500*time.Millisecond); err != nil {
		t.Fatalf("reaper failed: %v", err)
	}

	_ = cmd.Wait()
}

func TestReaperTermination_AbortsWhenSocketInodeMismatch(t *testing.T) {
	cmd := exec.Command("sleep", "10")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start test process: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	// "sleep" holds no socket fds at all, so any positive SocketInode value
	// should fail verification and abort the kill before any signal is sent.
	tun := model.Tunnel{
		PID:         cmd.Process.Pid,
		ProcessName: "sleep",
		SocketInode: 999999999,
	}

	err := reaper.TerminateTunnel("/proc", tun, 200*time.Millisecond)
	if err == nil {
		t.Fatalf("expected error due to socket inode mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "no longer holds socket inode") {
		t.Fatalf("expected inode mismatch error, got: %v", err)
	}

	// Verify the process was NOT killed: signal 0 should still succeed.
	if sigErr := cmd.Process.Signal(syscall.Signal(0)); sigErr != nil {
		t.Fatalf("expected process to still be alive after aborted kill, signal check failed: %v", sigErr)
	}
}
