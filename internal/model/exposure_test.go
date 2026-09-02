package model_test

import (
	"testing"

	"github.com/DiAndEn0/tunnel-snoop/internal/model"
)

func TestClassifyExposure(t *testing.T) {
	cases := []struct {
		name string
		addr string
		want model.Exposure
	}{
		{"ipv4 wildcard", "0.0.0.0", model.ExposureWildcard},
		{"ipv6 wildcard", "::", model.ExposureWildcard},
		{"ipv4 mapped wildcard", "::ffff:0.0.0.0", model.ExposureWildcard},

		{"ipv4 loopback", "127.0.0.1", model.ExposureLoopback},
		{"ipv4 loopback high", "127.13.9.2", model.ExposureLoopback},
		{"ipv6 loopback", "::1", model.ExposureLoopback},
		{"ipv4 mapped loopback", "::ffff:127.0.0.1", model.ExposureLoopback},

		{"rfc1918 ten", "10.4.5.6", model.ExposureLAN},
		{"rfc1918 172 low", "172.16.0.1", model.ExposureLAN},
		{"rfc1918 172 high", "172.31.255.254", model.ExposureLAN},
		{"rfc1918 192.168", "192.168.1.40", model.ExposureLAN},
		{"ipv4 link local", "169.254.10.1", model.ExposureLAN},
		{"ipv6 link local", "fe80::1", model.ExposureLAN},
		{"cgnat low", "100.64.0.1", model.ExposureLAN},
		{"cgnat high", "100.127.255.254", model.ExposureLAN},
		{"ipv6 unique local", "fd00::1", model.ExposureLAN},
		{"ipv4 mapped rfc1918", "::ffff:192.168.1.40", model.ExposureLAN},

		{"routable ipv4", "203.0.113.7", model.ExposurePublic},
		{"routable ipv6", "2001:db8::1", model.ExposurePublic},
		// Just outside the private ranges, to prove the boundaries are not
		// drawn on the first octet alone.
		{"just below 172.16/12", "172.15.0.1", model.ExposurePublic},
		{"just above 172.16/12", "172.32.0.1", model.ExposurePublic},
		{"just below cgnat", "100.63.255.255", model.ExposurePublic},
		{"just above cgnat", "100.128.0.0", model.ExposurePublic},

		// An address we cannot parse is an address we cannot vouch for, so it
		// must land on the loud side of the classification, never on [SAFE].
		{"unparseable", "not-an-address", model.ExposurePublic},
		{"empty", "", model.ExposurePublic},
		{"host and port", "127.0.0.1:5432", model.ExposurePublic},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := model.ClassifyExposure(c.addr); got != c.want {
				t.Fatalf("ClassifyExposure(%q) = %q, want %q", c.addr, got, c.want)
			}
		})
	}
}

// TestCheckExposureSurroundingWhitespace mirrors CheckWildcard, which trims
// before comparing: an address carrying stray whitespace must classify on its
// value, not fall through to the unparseable case.
func TestCheckExposureSurroundingWhitespace(t *testing.T) {
	tun := model.Tunnel{LocalAddress: "  10.0.0.5  "}
	if got := tun.CheckExposure(); got != model.ExposureLAN {
		t.Fatalf("expected LAN for padded address, got %q", got)
	}
}

// TestCheckExposureAgreesWithCheckWildcard guards the two classifiers against
// drifting apart: IsWildcard remains part of the JSON contract and is used by
// the client-counting logic, so anything CheckWildcard calls a wildcard must
// also be ExposureWildcard, and nothing else may be.
func TestCheckExposureAgreesWithCheckWildcard(t *testing.T) {
	for _, addr := range []string{"0.0.0.0", "::", "127.0.0.1", "::1", "192.168.1.10", "8.8.8.8", ""} {
		tun := model.Tunnel{LocalAddress: addr}
		if tun.CheckWildcard() != (tun.CheckExposure() == model.ExposureWildcard) {
			t.Fatalf("wildcard disagreement for %q: CheckWildcard=%v CheckExposure=%q",
				addr, tun.CheckWildcard(), tun.CheckExposure())
		}
	}
}
