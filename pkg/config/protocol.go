package config

import (
	"fmt"
)

// Protocol is an L4 protocol identifier used by frontends and targets.
type Protocol uint8

const (
	// ProtocolTCP identifies TCP.
	ProtocolTCP Protocol = 6
	// ProtocolUDP identifies UDP.
	ProtocolUDP Protocol = 17
)

// String returns the lowercase protocol name used in configuration and status
// output.
func (p Protocol) String() string {
	switch p {
	case ProtocolTCP:
		return "tcp"
	case ProtocolUDP:
		return "udp"
	default:
		return "invalid"
	}
}

// IsValid reports whether p is a supported protocol value.
func (p Protocol) IsValid() bool {
	return p.String() != "invalid"
}

// ParseProtocol parses a protocol string such as "tcp" or "udp".
func ParseProtocol(s string) (Protocol, error) {
	switch s {
	case "tcp":
		return ProtocolTCP, nil
	case "udp":
		return ProtocolUDP, nil
	default:
		return 0, fmt.Errorf("invalid Protocol %q", s)
	}
}

// MarshalText returns the lowercase protocol name.
func (p Protocol) MarshalText() ([]byte, error) {
	return []byte(p.String()), nil
}

// UnmarshalText decodes a protocol name such as "tcp" or "udp".
func (p *Protocol) UnmarshalText(text []byte) error {
	result, err := ParseProtocol(string(text))
	if err != nil {
		return err
	}
	*p = result
	return nil
}
