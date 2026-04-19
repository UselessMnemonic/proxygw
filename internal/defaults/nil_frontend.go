package defaults

import (
	"net/netip"
	"proxygw/internal/frontend"
	"proxygw/pkg/config"
)

type NilFrontend struct{}

func (NilFrontend) Name() string {
	return "nil"
}

func (NilFrontend) New(config.Protocol, netip.AddrPort, map[string]any) (frontend.Driver, error) {
	return NilFrontend{}, nil
}

func (NilFrontend) Kind() frontend.Kind {
	return NilFrontend{}
}

func (NilFrontend) Start() error {
	return nil
}

func (NilFrontend) Stop() error {
	return nil
}

func (NilFrontend) Close() {}
