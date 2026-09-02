package model

import (
	"net"
	"strings"
)

// Exposure describes how far a tunnel's listening address can be reached from.
// It is a coarser question than "is this a wildcard bind": a listener on
// 192.168.1.40 is not a wildcard, yet it is reachable by every machine on the
// LAN, which is the surprise this type exists to surface.
type Exposure string

const (
	// ExposureLoopback is a bind reachable only from this host (127.0.0.0/8,
	// ::1). This is the only genuinely safe tier.
	ExposureLoopback Exposure = "loopback"

	// ExposureLAN is a bind on an address that is routable within a local or
	// carrier-shared network but not on the open internet.
	ExposureLAN Exposure = "lan"

	// ExposurePublic is a bind on a globally routable address. It is also the
	// verdict for any address we could not parse, on the principle that an
	// unknown binding is not a safe binding.
	ExposurePublic Exposure = "public"

	// ExposureWildcard is a bind to the unspecified address (0.0.0.0, ::),
	// which accepts on every interface the host currently has, including ones
	// added after the tunnel started.
	ExposureWildcard Exposure = "wildcard"
)

// ClassifyExposure sorts a listening address into an Exposure tier.
//
// An address that does not parse is reported as ExposurePublic rather than
// being skipped or defaulted to loopback. Getting this backwards would paint a
// [SAFE] badge on a binding nobody has actually vetted, and a missed warning is
// far more expensive here than a spurious one.
func ClassifyExposure(addr string) Exposure {
	ip := net.ParseIP(strings.TrimSpace(addr))
	if ip == nil {
		return ExposurePublic
	}

	// Checked first: the unspecified address is neither loopback nor private,
	// and it is a strictly wider exposure than either.
	if ip.IsUnspecified() {
		return ExposureWildcard
	}

	if ip.IsLoopback() {
		return ExposureLoopback
	}

	// IsPrivate covers RFC 1918 (10/8, 172.16/12, 192.168/16) and IPv6 unique
	// local addresses (fc00::/7). Link-local covers 169.254/16 and fe80::/10.
	if ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || isCGNAT(ip) {
		return ExposureLAN
	}

	return ExposurePublic
}

// isCGNAT reports whether ip falls inside RFC 6598 shared address space
// (100.64.0.0/10). The standard library does not treat this range as private,
// but a host addressable there sits behind a carrier's NAT rather than on the
// open internet, so it belongs with the LAN tier.
func isCGNAT(ip net.IP) bool {
	v4 := ip.To4()
	return v4 != nil && v4[0] == 100 && v4[1]&0xc0 == 0x40
}
