package config

import (
	"fmt"
	"net/netip"
	"strings"
)

// Target defines a backend target and how it is activated.
type Target struct {
	Name        string           `yaml:"name"`
	Kind        string           `yaml:"kind"`
	IdleTimeout TTL              `yaml:"idle_timeout"`
	Endpoints   []TargetEndpoint `yaml:"endpoints"`
	Options     map[string]any   `yaml:"options"`
}

// TargetEndpoint defines an addressable backend service on a target.
type TargetEndpoint struct {
	Name     string         `yaml:"name"`
	Protocol Protocol       `yaml:"protocol"`
	Address  netip.AddrPort `yaml:"address"`
}

type TargetEndpointReference struct {
	TargetName   string `yaml:"target_name"`
	EndpointName string `yaml:"endpoint_name"`
}

func (e TargetEndpointReference) MarshalText() ([]byte, error) {
	return []byte(fmt.Sprintf("%s/%s", e.TargetName, e.EndpointName)), nil
}

func (e *TargetEndpointReference) UnmarshalText(text []byte) error {
	parts := strings.Split(string(text), "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid endpoint reference: %q", text)
	}
	*e = TargetEndpointReference{
		TargetName:   parts[0],
		EndpointName: parts[1],
	}
	return nil
}
