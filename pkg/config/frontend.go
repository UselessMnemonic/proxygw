package config

import (
	"net/netip"
)

// Frontend defines a listening socket and forwarding behavior.
type Frontend struct {
	Name        string                  `yaml:"name"`
	Kind        string                  `yaml:"kind"`
	Protocol    Protocol                `yaml:"protocol"`
	Listen      netip.AddrPort          `yaml:"listen"`
	FlowTimeout TTL                     `yaml:"flow_timeout"`
	Endpoint    TargetEndpointReference `yaml:"target"`
	Options     map[string]any          `yaml:"options"`
}
