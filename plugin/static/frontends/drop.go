package frontends

import (
	"net/netip"
	"proxygw/internal/frontend"
	"proxygw/pkg/config"
)

type DropHandler chan struct{}

func (DropHandler) Start() error {
	return nil
}

func (DropHandler) Close() error {
	return nil
}

func (d DropHandler) ShouldWarm() <-chan struct{} {
	return d
}

func NewDropHandler(name string, protocol config.Protocol, address netip.AddrPort, options map[string]any) (frontend.Handler, error) {
	ch := make(chan struct{}, 1)
	ch <- struct{}{}
	return DropHandler(ch), nil
}
