package config

import "net/netip"

// Target defines a backend target and how it is activated.
type Target struct {
	Name        string             `yaml:"name"`
	Kind        NamespaceReference `yaml:"kind"`
	IdleTimeout TTL                `yaml:"idle_timeout"`
	Endpoints   []TargetEndpoint   `yaml:"endpoints"`
	Options     map[string]any     `yaml:"options"`
}

// TargetEndpoint defines an addressable backend service on a target.
type TargetEndpoint struct {
	Name     string         `yaml:"name"`
	Protocol Protocol       `yaml:"protocol"`
	Address  netip.AddrPort `yaml:"address"`
}
