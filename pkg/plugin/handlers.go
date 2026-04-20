package plugin

import (
	"context"
	"net/netip"
	"proxygw/pkg/config"
)

type InitHandler func(options map[string]any) error

type ShutdownHandler func(context.Context) error

type TargetHandler interface {
	New(name string, kind string, options map[string]any) error
	Warm(name string) error
	Drain(name string) error
	Close(name string) error
}

type FrontendHandler interface {
	New(name string, kind string, protocol config.Protocol, listen netip.AddrPort, options map[string]any) error
	Start(name string) error
	Stop(name string) error
	Close(name string) error
}
