package procfs

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/DiAndEn0/tunnel-snoop/internal/model"
)

// ParseSockets reads the TCP and TCP6 socket tables under procRoot/net and
// returns the parsed socket entries. procRoot is typically "/proc" but may
// be overridden (e.g. in tests) to point at a fixture directory.
//
// A single unreadable table is tolerated, since hosts with IPv6 disabled have
// no net/tcp6 and IPv4-less namespaces have no net/tcp. If neither table can be
// read the socket view is empty for want of data rather than because nothing is
// listening, so an error is returned instead of a silently empty slice.
func ParseSockets(procRoot string) ([]model.SocketEntry, error) {
	var entries []model.SocketEntry

	v4Entries, v4Err := parseSocketFile(filepath.Join(procRoot, "net", "tcp"), model.ProtoIPv4)
	if v4Err == nil {
		entries = append(entries, v4Entries...)
	}

	v6Entries, v6Err := parseSocketFile(filepath.Join(procRoot, "net", "tcp6"), model.ProtoIPv6)
	if v6Err == nil {
		entries = append(entries, v6Entries...)
	}

	if v4Err != nil && v6Err != nil {
		return nil, fmt.Errorf("no readable socket table under %s: %w",
			filepath.Join(procRoot, "net"), errors.Join(v4Err, v6Err))
	}

	return entries, nil
}

func parseSocketFile(path string, proto model.Protocol) ([]model.SocketEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	var results []model.SocketEntry
	scanner := bufio.NewScanner(file)

	_ = scanner.Scan() // discard the column header line

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 10 {
			continue
		}

		localAddrStr := fields[1]
		remAddrStr := fields[2]
		stateHex := fields[3]
		inodeStr := fields[9]

		localIP, localPort, err := parseHexAddr(localAddrStr, proto)
		if err != nil {
			continue
		}

		remIP, remPort, err := parseHexAddr(remAddrStr, proto)
		if err != nil {
			continue
		}

		inode, err := strconv.ParseUint(inodeStr, 10, 64)
		if err != nil {
			continue
		}

		var state model.SocketState
		switch stateHex {
		case "0A":
			state = model.StateListen
		case "01":
			state = model.StateEstablished
		default:
			continue
		}

		results = append(results, model.SocketEntry{
			Protocol:   proto,
			LocalIP:    localIP,
			LocalPort:  localPort,
			RemoteIP:   remIP,
			RemotePort: remPort,
			State:      state,
			Inode:      inode,
		})
	}

	return results, scanner.Err()
}

func parseHexAddr(addr string, proto model.Protocol) (string, int, error) {
	parts := strings.Split(addr, ":")
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("invalid address format: %s", addr)
	}

	port64, err := strconv.ParseInt(parts[1], 16, 32)
	if err != nil {
		return "", 0, err
	}
	port := int(port64)

	ipHex := parts[0]
	if proto == model.ProtoIPv4 {
		b, err := hex.DecodeString(ipHex)
		if err != nil || len(b) != 4 {
			return "", 0, fmt.Errorf("invalid ipv4 hex: %s", ipHex)
		}
		// Little-endian decoded to net.IPv4
		ip := net.IPv4(b[3], b[2], b[1], b[0])
		return ip.String(), port, nil
	}

	// IPv6 is 16 bytes encoded as 4 32-bit words in host endian
	b, err := hex.DecodeString(ipHex)
	if err != nil || len(b) != 16 {
		return "", 0, fmt.Errorf("invalid ipv6 hex: %s", ipHex)
	}

	var ipBytes [16]byte
	for i := 0; i < 4; i++ {
		w := binary.LittleEndian.Uint32(b[i*4 : (i+1)*4])
		binary.BigEndian.PutUint32(ipBytes[i*4:(i+1)*4], w)
	}

	ip := net.IP(ipBytes[:])
	return ip.String(), port, nil
}
