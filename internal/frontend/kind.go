package frontend

import (
	"net/netip"
	"proxygw/pkg/config"
)

type Kind interface {
	Name() string
	New(config.Protocol, netip.AddrPort, map[string]any) (Driver, error)
}
