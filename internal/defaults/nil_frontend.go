package defaults

import (
	"net/netip"
	"proxygw/internal/frontend"
	"proxygw/pkg/config"
)

var NilFrontend frontend.Kind = nilFrontend{}

type nilFrontend struct{}

type nilDriver <-chan struct{}

func (nilFrontend) Name() string {
	return "nil"
}

func (nilFrontend) New(config.Protocol, netip.AddrPort, map[string]any) (frontend.Driver, error) {
	result := make(chan struct{}, 1)
	result <- struct{}{}
	return nilDriver(result), nil
}

func (nilDriver) Kind() frontend.Kind {
	return NilFrontend
}

func (nilDriver) Start() error {
	return nil
}

func (nilDriver) Stop() error {
	return nil
}

func (self nilDriver) ShouldWarm() <-chan struct{} {
	return self
}

func (nilDriver) Close() {}
