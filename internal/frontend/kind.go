package frontend

import (
	"net/netip"
	"proxygw/pkg/config"
)

// Kind identifies a frontend implementation and constructs drivers for named listeners.
type Kind interface {
	Name() string
	New(name string, protocol config.Protocol, port netip.AddrPort, options map[string]any) (Driver, error)
}
