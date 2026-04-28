package target

import (
	"net/netip"

	"github.com/UselessMnemonic/proxygw/pkg/config"
)

// Endpoint represents a specific endpoint on which a target expects to receive traffic.
type Endpoint struct {
	Name     string
	Address  netip.AddrPort
	Protocol config.Protocol
}

// IsValid determines if the Endpoint is valid. A valid Endpoint must be fully specified.
func (this *Endpoint) IsValid() bool {
	return this.Name != "" && this.Address.IsValid() &&
		this.Address.Port() != 0 && !this.Address.Addr().IsUnspecified()
}

// Overlaps determines if two Endpoints conflict, according to the following rules:
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

	// if both ports are different, there is no chance of overlap (:443 vs :80)
	if this.Address.Port() != other.Address.Port() {
		return false
	}

	// only other check is if the two addresses are literally the same
	return other.Address.Addr() == this.Address.Addr()
}
