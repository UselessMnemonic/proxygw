package dataplane

import (
	"fmt"
	"net/netip"

	"github.com/UselessMnemonic/proxygw/pkg/config"
)

// DNATMapping defines to where a host endpoint shall be forwarded, including for what protocol.
// This type does not support IPv6 zones, therefore disallows link-local addresses.
type DNATMapping struct {
	// Source is the host address and port, like 10.0.0.10:22, including wildcard addresses like [::]:443
	Source netip.AddrPort
	// Destination is the remote address and port, which cannot be any wildcard address
	Destination netip.AddrPort
	// FlowTimeout determines the input conntrack timeout for host traffic to Source.
	// A low timeout value ensures that a well-behaved client is forwarded to the destination sooner.
	// A timeout of 0 causes conntrack to use the system default.
	FlowTimeout config.TTL
	// Protocol specifies the L4 protocol of the mapping, like UDP or TCP.
	Protocol config.Protocol
}

// Validate returns an error if the mapping is invalid
func (dm *DNATMapping) Validate() error {
	if !dm.Protocol.IsValid() {
		return fmt.Errorf("invalid protocol: %v", dm.Protocol)
	}
	if !dm.Source.IsValid() {
		return fmt.Errorf("invalid source: %s", dm.Source.String())
	}
	if !dm.Destination.IsValid() {
		return fmt.Errorf("invalid destination: %s", dm.Destination.String())
	}

	if dm.Source.Port() == 0 {
		return fmt.Errorf("source cannot have port 0: %s", dm.Source.String())
	}
	if dm.Destination.Port() == 0 {
		return fmt.Errorf("destination cannot have port 0: %s", dm.Destination.String())
	}

	// TODO: add support for link-local addresses, and zone-scoped unspecified addresses
	if dm.Source.Addr().Zone() != "" {
		return fmt.Errorf("source cannot have zone: %s", dm.Source.String())
	}
	if dm.Destination.Addr().Zone() != "" {
		return fmt.Errorf("destination cannot have zone: %s", dm.Destination.String())
	}

	// only source address may be unspecified
	if dm.Destination.Addr().IsUnspecified() {
		return fmt.Errorf("destination cannot be unspecified: %s", dm.Destination.String())
	}

	if (dm.Source.Addr().Is6() && dm.Destination.Addr().Is4()) || (dm.Source.Addr().Is4() && dm.Destination.Addr().Is6()) {
		return fmt.Errorf("source and destination must both be either IPv4 or IPv6: %s", dm.Destination.String())
	}
	if dm.Source == dm.Destination {
		return fmt.Errorf("source and destination must be different: %s", dm.Source.String())
	}
	return nil
}

// Overlaps determines if two mappings overlap according to the following rules:
// (1) No two IP addresses may appear in either Source or Destination simultaneously;
// (2) The wildcard Source IPv4 address owns its port in the IPv4 address space;
// (3) The wildcard Source IPv6 address owns its port in the IPv6 address space
func (this *DNATMapping) Overlaps(other *DNATMapping) bool {
	// if both protocols are different, there is no chance of overlap (udp vs tcp)
	if this.Protocol != other.Protocol {
		return false
	}

	// if neither mapping is the same type, there is no chance of overlap (ipv4 vs ipv6)
	if this.Source.Addr().Is4() != other.Source.Addr().Is4() {
		return false
	}

	// ensure no address appears in the other mapping
	if this.Source == other.Source || this.Source == other.Destination {
		return true
	}
	if this.Destination == other.Destination || this.Destination == other.Source {
		return true
	}

	// if both ports are different, there is no chance of overlap (0.0.0.0:443 vs 10.0.0.5:80)
	if this.Source.Port() != other.Source.Port() {
		return false
	}

	// same protocol, same ip type, same port, so if one of the addresses is a wildcard then we definitely overlay
	if this.Source.Addr().IsUnspecified() || other.Source.Addr().IsUnspecified() {
		return true
	}

	// same protocol, same ip type, same port, but different source addresses
	return false
}
