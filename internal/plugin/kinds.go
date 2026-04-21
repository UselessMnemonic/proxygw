package plugin

import (
	"net/netip"
	"proxygw/internal/frontend"
	"proxygw/internal/target"
	"proxygw/pkg/config"
)

type pluginFrontendKind struct {
	host *Host
	name string
}

func (k pluginFrontendKind) Name() string {
	return k.name
}

func (k pluginFrontendKind) New(name string, protocol config.Protocol, port netip.AddrPort, options map[string]any) (frontend.Driver, error) {
	return k.host.newFrontendDriver(k, name, protocol, port, options)
}

func (h *Host) newFrontendKind(name string) frontend.Kind {
	return pluginFrontendKind{h, name}
}

type pluginTargetKind struct {
	host *Host
	name string
}

func (k pluginTargetKind) Name() string {
	return k.name
}

func (k pluginTargetKind) New(name string, options map[string]any) (target.Driver, error) {
	return k.host.newTargetDriver(k, name, options)
}

func (h *Host) newTargetKind(name string) target.Kind {
	return pluginTargetKind{h, name}
}
