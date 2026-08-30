package procfs

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/DiAndEn0/tunnel-snoop/internal/model"
)

// FindTunnels scans procRoot for processes whose binary name matches
// allowedBinaries and correlates their open file descriptors against the
// LISTEN sockets present in sockets, producing a Tunnel for each match.
func FindTunnels(procRoot string, sockets []model.SocketEntry, allowedBinaries []string) ([]model.Tunnel, error) {
	listenMap := make(map[uint64]model.SocketEntry)
	for _, s := range sockets {
		if s.State == model.StateListen {
			listenMap[s.Inode] = s
		}
	}

	entries, err := os.ReadDir(procRoot)
	if err != nil {
		return nil, err
	}

	binarySet := make(map[string]bool)
	for _, b := range allowedBinaries {
		binarySet[strings.ToLower(b)] = true
	}

	var tunnels []model.Tunnel

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}

		pidDir := filepath.Join(procRoot, entry.Name())

		// Read comm
		commBytes, err := os.ReadFile(filepath.Join(pidDir, "comm"))
		if err != nil {
			continue
		}
		comm := strings.TrimSpace(string(commBytes))

		if !binarySet[strings.ToLower(comm)] {
			continue
		}

		// Read cmdline
		cmdlineBytes, _ := os.ReadFile(filepath.Join(pidDir, "cmdline"))
		cmdline := string(bytes.ReplaceAll(cmdlineBytes, []byte{0}, []byte(" ")))

		// Check open file descriptors for socket inodes
		fdDir := filepath.Join(pidDir, "fd")
		fds, err := os.ReadDir(fdDir)
		if err != nil {
			continue
		}

		for _, fd := range fds {
			link, err := os.Readlink(filepath.Join(fdDir, fd.Name()))
			if err != nil {
				continue
			}

			if strings.HasPrefix(link, "socket:[") && strings.HasSuffix(link, "]") {
				inodeStr := link[len("socket:[") : len(link)-1]
				inode, err := strconv.ParseUint(inodeStr, 10, 64)
				if err != nil {
					continue
				}

				if sock, found := listenMap[inode]; found {
					tun := model.Tunnel{
						PID:          pid,
						ProcessName:  comm,
						CommandLine:  strings.TrimSpace(cmdline),
						LocalAddress: sock.LocalIP,
						LocalPort:    sock.LocalPort,
						Protocol:     sock.Protocol,
						SocketInode:  inode,
						FirstSeen:    time.Now(),
						LastActive:   time.Now(),
					}
					tun.IsWildcard = tun.CheckWildcard()
					tunnels = append(tunnels, tun)
				}
			}
		}
	}

	return tunnels, nil
}
