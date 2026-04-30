package dataplane

import (
	"fmt"
	"net/netip"

	"github.com/UselessMnemonic/proxygw/pkg/config"
)

// Mapping defines which host endpoint forwards to which target endpoint, given a specific protocol.
// Mappings per protocol are one-to-one, where an IP cannot be both Source and Destination.
// More formally, if Dataplane represents a function
//
// Dataplane : Protocol -> f : Source -> Destination,
//
// then for any Protocol,
//
// domain(f) is disjoint with codomain(f).
//
// In addition, given a specific protocol and port, if the Source of any Mapping includes the unspecified address,
// then no other source may use the same port.
// This type does not support IPv6 zones, therefore disallows link-local addresses.
type Mapping struct {
	// Source is the host address and port, like 10.0.0.10:22, including wildcard addresses like [::]:443
	Source netip.AddrPort
	// Destination is the remote address and port, which cannot be any wildcard address
	Destination netip.AddrPort
	// Timeout determines the length of time a flow matching Source will remain valid after the next packet.
	// When DNAT is toggled for this mapping, existing flows may continue to be served by the host or forwarded.
	// A low timeout value ensures that a well-behaved client reaches the correct destination sooner.
	Timeout config.TTL
	// Protocol specifies the L4 protocol of the mapping, like UDP or TCP.
	Protocol config.Protocol
}

// Validate returns an error if the mapping is invalid
func (m *Mapping) Validate() error {
	if !m.Protocol.IsValid() {
		return fmt.Errorf("invalid protocol: %v", m.Protocol)
	}
	if !m.Source.IsValid() {
		return fmt.Errorf("invalid source: %s", m.Source.String())
	}
	if !m.Destination.IsValid() {
		return fmt.Errorf("invalid destination: %s", m.Destination.String())
	}

	if m.Source.Port() == 0 {
		return fmt.Errorf("source cannot have port 0: %s", m.Source.String())
	}
	if m.Destination.Port() == 0 {
		return fmt.Errorf("destination cannot have port 0: %s", m.Destination.String())
	}

	// TODO: add support for link-local addresses, and zone-scoped unspecified addresses
	if m.Source.Addr().Zone() != "" {
		return fmt.Errorf("source cannot have zone: %s", m.Source.String())
	}
	if m.Destination.Addr().Zone() != "" {
		return fmt.Errorf("destination cannot have zone: %s", m.Destination.String())
	}

	// only source address may be unspecified
	if m.Destination.Addr().IsUnspecified() {
		return fmt.Errorf("destination cannot be unspecified: %s", m.Destination.String())
	}

	if (m.Source.Addr().Is6() && m.Destination.Addr().Is4()) || (m.Source.Addr().Is4() && m.Destination.Addr().Is6()) {
		return fmt.Errorf("source and destination must both be either IPv4 or IPv6: %s", m.Destination.String())
	}
	if m.Source == m.Destination {
		return fmt.Errorf("source and destination must be different: %s", m.Source.String())
	}
	return nil
}

// Overlaps determines if two mappings overlap according to the following rules:
// (1) No two IP addresses may appear in either Source or Destination simultaneously;
// (2) The wildcard Source IPv4 address owns its port in the IPv4 address space;
// (3) The wildcard Source IPv6 address owns its port in the IPv6 address space
func (m *Mapping) Overlaps(other *Mapping) bool {
	// if both protocols are different, there is no chance of overlap (udp vs tcp)
	if m.Protocol != other.Protocol {
		return false
	}

	// if neither mapping is the same type, there is no chance of overlap (ipv4 vs ipv6)
	if m.Source.Addr().Is4() != other.Source.Addr().Is4() {
		return false
	}

	// ensure no address appears in the other mapping
	if m.Source == other.Source || m.Source == other.Destination {
		return true
	}
	if m.Destination == other.Destination || m.Destination == other.Source {
		return true
	}

	// if both ports are different, there is no chance of overlap (0.0.0.0:443 vs 10.0.0.5:80)
	if m.Source.Port() != other.Source.Port() {
		return false
	}

	// same protocol, same ip type, same port, so if one of the addresses is a wildcard then we definitely overlay
	if m.Source.Addr().IsUnspecified() || other.Source.Addr().IsUnspecified() {
		return true
	}

	// same protocol, same ip type, same port, but different source addresses
	return false
}
