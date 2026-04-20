package frontend

import (
	"net/netip"
	"proxygw/pkg/config"
)

type Kind interface {
	Name() string
	New(string, config.Protocol, netip.AddrPort, map[string]any) (Driver, error)
}
