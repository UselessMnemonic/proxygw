package target

import (
	"net/netip"
	"proxygw/pkg/config"
)

type Endpoint struct {
	Name     string
	Address  netip.AddrPort
	Protocol config.Protocol
}

// Overlaps determines if two Endpoints conflict according to the following rules:
// (1) No two Endpoints may have the same name
// (2) The wildcard IPv4 address owns its port in the IPv4 address space;
// (3) The wildcard IPv6 address owns its port in the IPv6 address space
func (this *Endpoint) Overlaps(other *Endpoint) bool {
	if this.Name == other.Name {
		return true
	}

	// if both protocols are different, there is no chance of overlap (udp vs tcp)
	if this.Protocol != other.Protocol {
		return false
	}

	// if neither address type is the same type, there is no chance of overlap (ipv4 vs ipv6)
	if this.Address.Addr().Is4() != other.Address.Addr().Is4() {
		return false
	}

	// if both ports are different, there is no chance of overlap (0.0.0.0:443 vs 10.0.0.5:80)
	if this.Address.Port() != other.Address.Port() {
		return false
	}

	// same protocol, same ip type, same port, so if one of the addresses is a wildcard then we definitely overlay
	if this.Address.Addr().IsUnspecified() || other.Address.Addr().IsUnspecified() {
		return true
	}

	// only other check is if the two addresses are literally the same
	return other.Address.Addr() == this.Address.Addr()
}
