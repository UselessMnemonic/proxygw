package frontend

import (
	"net/netip"

	"github.com/UselessMnemonic/proxygw/pkg/config"
)

// Handler manages the lifecycle of a frontend.
type Handler interface {
	// Start invokes the handler to start listening for traffic, blocking until ready.
	Start() error
	// Stop invokes the handler to stop listening for traffic, blocking until done.
	Stop() error
	// Close tears down the handler, blocking until done.
	Close() error
	// ShouldWarm returns a channel that signals when the handler should be warmed.
	ShouldWarm() <-chan struct{}
}

// HandlerCtor is a function that creates a new frontend.
type HandlerCtor func(name string, protocol config.Protocol, address netip.AddrPort, options map[string]any) (Handler, error)
