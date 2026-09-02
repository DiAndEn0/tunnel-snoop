package model

import (
	"strings"
	"time"
)

type Protocol string

const (
	ProtoIPv4 Protocol = "tcp"
	ProtoIPv6 Protocol = "tcp6"
)

type SocketState string

const (
	StateListen      SocketState = "LISTEN"
	StateEstablished SocketState = "ESTABLISHED"
)

type SocketEntry struct {
	Protocol   Protocol
	LocalIP    string
	LocalPort  int
	RemoteIP   string
	RemotePort int
	State      SocketState
	Inode      uint64
}

type Tunnel struct {
	PID           int       `json:"pid"`
	ProcessName   string    `json:"process_name"`
	CommandLine   string    `json:"command_line"`
	LocalAddress  string    `json:"local_address"`
	LocalPort     int       `json:"local_port"`
	Protocol      Protocol  `json:"protocol"`
	SocketInode   uint64    `json:"socket_inode"`
	IsWildcard    bool      `json:"is_wildcard"`
	FirstSeen     time.Time `json:"first_seen"`
	LastActive    time.Time `json:"last_active"`
	ActiveClients int       `json:"active_clients"`

	// BytesRead and BytesWritten carry the process's cumulative rchar/wchar
	// counters from /proc/<pid>/io: every byte moved through the read(2) and
	// write(2) syscall families, sockets included. They are deliberately not
	// the read_bytes/write_bytes block-device counters, because a tunnel
	// forwards network traffic that never touches a disk — against the block
	// counters a fully saturated tunnel is indistinguishable from a dead one.
	// The JSON names are kept as-is so existing consumers of the -json output
	// do not break.
	BytesRead    uint64 `json:"bytes_read"`
	BytesWritten uint64 `json:"bytes_written"`

	IdleDuration time.Duration `json:"idle_duration"`
}

func (t *Tunnel) CheckWildcard() bool {
	addr := strings.TrimSpace(t.LocalAddress)
	return addr == "0.0.0.0" || addr == "::"
}

func (t *Tunnel) CalculateIdle(now time.Time) time.Duration {
	if t.LastActive.IsZero() {
		return 0
	}
	return now.Sub(t.LastActive)
}
