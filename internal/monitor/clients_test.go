package monitor

import (
	"testing"

	"github.com/DiAndEn0/tunnel-snoop/internal/model"
)

func established(ip string, port int) model.SocketEntry {
	return model.SocketEntry{
		Protocol:  model.ProtoIPv4,
		LocalIP:   ip,
		LocalPort: port,
		State:     model.StateEstablished,
	}
}

// buildCounts mirrors the aggregation Reconcile performs, so the counting rules
// can be exercised without a synthetic procfs.
func buildCounts(sockets []model.SocketEntry) map[portKey]map[string]int {
	counts := make(map[portKey]map[string]int)
	for _, s := range sockets {
		if s.State != model.StateEstablished {
			continue
		}
		key := portKey{proto: s.Protocol, port: s.LocalPort}
		if counts[key] == nil {
			counts[key] = make(map[string]int)
		}
		counts[key][s.LocalIP]++
	}
	return counts
}

// A loopback-bound tunnel must not absorb connections terminating on another
// address. Counting by port alone reports 3 here, which makes an idle tunnel
// look busy and exempts it from -kill-idle indefinitely.
func TestCountClientsIgnoresOtherLocalAddresses(t *testing.T) {
	counts := buildCounts([]model.SocketEntry{
		established("127.0.0.1", 5432),
		established("192.168.1.10", 5432),
		established("10.0.0.4", 5432),
	})

	tun := model.Tunnel{
		Protocol:     model.ProtoIPv4,
		LocalAddress: "127.0.0.1",
		LocalPort:    5432,
	}

	if got := countClients(counts, tun); got != 1 {
		t.Fatalf("expected 1 client on the bound address, got %d", got)
	}
}

// A wildcard listener accepts on every interface, and the accepted sockets carry
// the specific interface address rather than 0.0.0.0, so all of them count.
func TestCountClientsSumsAllAddressesForWildcardListener(t *testing.T) {
	counts := buildCounts([]model.SocketEntry{
		established("192.168.1.10", 6379),
		established("10.0.0.4", 6379),
	})

	tun := model.Tunnel{
		Protocol:     model.ProtoIPv4,
		LocalAddress: "0.0.0.0",
		LocalPort:    6379,
		IsWildcard:   true,
	}

	if got := countClients(counts, tun); got != 2 {
		t.Fatalf("expected both interfaces counted for a wildcard listener, got %d", got)
	}
}

// Protocol remains part of the key: a TCP and TCP6 listener on the same port
// number are distinct endpoints.
func TestCountClientsSeparatesProtocols(t *testing.T) {
	counts := buildCounts([]model.SocketEntry{
		established("127.0.0.1", 8080),
		{
			Protocol:  model.ProtoIPv6,
			LocalIP:   "::1",
			LocalPort: 8080,
			State:     model.StateEstablished,
		},
	})

	tun := model.Tunnel{
		Protocol:     model.ProtoIPv6,
		LocalAddress: "::1",
		LocalPort:    8080,
	}

	if got := countClients(counts, tun); got != 1 {
		t.Fatalf("expected only the IPv6 connection, got %d", got)
	}
}

func TestCountClientsReturnsZeroForUnknownEndpoint(t *testing.T) {
	counts := buildCounts([]model.SocketEntry{established("127.0.0.1", 5432)})

	tun := model.Tunnel{
		Protocol:     model.ProtoIPv4,
		LocalAddress: "127.0.0.1",
		LocalPort:    9999,
	}

	if got := countClients(counts, tun); got != 0 {
		t.Fatalf("expected no clients for an unlistened port, got %d", got)
	}
}
