package plugin

import (
	"net/netip"
	"proxygw/internal/frontend"
	"proxygw/internal/target"
	"proxygw/pkg/config"
)

type pluginFrontendDriver struct {
	kind       pluginFrontendKind
	shouldWarm chan struct{}
}

func (d pluginFrontendDriver) Kind() frontend.Kind {
	return d.kind
}

func (d pluginFrontendDriver) Start() error {
	// TODO: make RPC call
}

func (d pluginFrontendDriver) Stop() error {
	// TODO: make RPC call
}

func (d pluginFrontendDriver) ShouldWarm() <-chan struct{} {
	return d.shouldWarm
}

func (d pluginFrontendDriver) Close() {
	// TODO: make RPC call and then deregister the driver
}

func (h *Host) newFrontendDriver(kind pluginFrontendKind, name string, protocol config.Protocol, port netip.AddrPort, options map[string]any) (frontend.Driver, error) {
	// TODO: make RPC call and then register the driver
}

type pluginTargetDriver struct {
	kind pluginTargetKind
}

func (d pluginTargetDriver) Kind() target.Kind {
	return d.kind
}

func (d pluginTargetDriver) Warm() error {
	// TODO: make RPC call
}

func (d pluginTargetDriver) Drain() error {
	// TODO: make RPC call
}

func (d pluginTargetDriver) Close() {
	// TODO: make RPC call and then deregister the driver
}

func (h *Host) newTargetDriver(kind pluginTargetKind, name string, options map[string]any) (target.Driver, error) {
	// TODO: make RPC call and then register the driver
}
