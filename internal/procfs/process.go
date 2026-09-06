package procfs

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/DiAndEn0/tunnel-snoop/internal/model"
)

// FindTunnels scans procRoot for processes whose binary name matches
// allowedBinaries and correlates their open file descriptors against the
// LISTEN sockets present in sockets, producing a Tunnel for each match.
//
// Only processes owned by uid are considered. Pass a negative uid to scan every
// process regardless of owner. Ownership is taken from the owner of the
// /proc/<pid> directory.
//
// The filter matters when running with elevated privileges. Unprivileged, the
// fd traversal below already fails with EACCES on other users' processes, so
// the result is the same either way; as root it is the only thing preventing
// user A from seeing or terminating user B's tunnels.
func FindTunnels(procRoot string, sockets []model.SocketEntry, allowedBinaries []string, uid int) ([]model.Tunnel, error) {
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

	allowed := make(map[string]bool, len(allowedBinaries))
	for _, b := range allowedBinaries {
		allowed[strings.ToLower(b)] = true
	}

	now := time.Now()
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

		if uid >= 0 && !ownedBy(pidDir, uid) {
			continue
		}

		commBytes, err := os.ReadFile(filepath.Join(pidDir, "comm"))
		if err != nil {
			continue
		}
		comm := strings.TrimSpace(string(commBytes))
		if !allowed[strings.ToLower(comm)] {
			continue
		}

		cmdlineBytes, _ := os.ReadFile(filepath.Join(pidDir, "cmdline"))
		cmdline := strings.ReplaceAll(string(cmdlineBytes), "\x00", " ")
		cmdline = strings.TrimSpace(cmdline)

		fdDir := filepath.Join(pidDir, "fd")
		fdEntries, err := os.ReadDir(fdDir)
		if err != nil {
			continue
		}

		seenInodes := make(map[uint64]bool)

		for _, fdEntry := range fdEntries {
			link, err := os.Readlink(filepath.Join(fdDir, fdEntry.Name()))
			if err != nil {
				continue
			}

			if strings.HasPrefix(link, "socket:[") && strings.HasSuffix(link, "]") {
				inodeStr := link[8 : len(link)-1]
				inode, err := strconv.ParseUint(inodeStr, 10, 64)
				if err != nil {
					continue
				}

				// A single socket fd can be dup'd across multiple fd slots
				// (e.g. stdout/stderr redirection, dup2). seenInodes prevents
				// producing duplicate Tunnel entries for the same listening
				// socket within the same process.
				if seenInodes[inode] {
					continue
				}
				seenInodes[inode] = true

				if sock, ok := listenMap[inode]; ok {
					tun := model.Tunnel{
						PID:          pid,
						ProcessName:  comm,
						CommandLine:  cmdline,
						LocalAddress: sock.LocalIP,
						LocalPort:    sock.LocalPort,
						Protocol:     sock.Protocol,
						SocketInode:  inode,
						FirstSeen:    now,
						LastActive:   now,
					}
					tun.IsWildcard = tun.CheckWildcard()
					tun.Exposure = tun.CheckExposure()
					tunnels = append(tunnels, tun)
				}
			}
		}
	}

	return tunnels, nil
}

// ownedBy reports whether pidDir belongs to uid. It reports false when
// ownership cannot be determined so an unstattable entry is skipped.
func ownedBy(pidDir string, uid int) bool {
	info, err := os.Stat(pidDir)
	if err != nil {
		return false
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return false
	}
	return int(stat.Uid) == uid
}
