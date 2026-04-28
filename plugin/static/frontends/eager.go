package frontends

import (
	"net/netip"
	"proxygw/pkg/config"
	"proxygw/pkg/frontend"
	"time"
)

const eagerWarmInterval = time.Second

type eagerCommand uint8

const (
	eagerStart eagerCommand = iota
	eagerStop
	eagerClose
)

// EagerHandler is a frontend driver that periodically asks its target to warm
// while the frontend is running.
type EagerHandler struct {
	ch       chan struct{}
	commands chan eagerCommand
	closed   bool
}

// Start begins emitting warm signals.
func (h *EagerHandler) Start() error {
	if h.closed {
		return nil
	}
	h.commands <- eagerStart
	return nil
}

// Stop pauses warm signals.
func (h *EagerHandler) Stop() error {
	if h.closed {
		return nil
	}
	h.commands <- eagerStop
	return nil
}

// Close permanently stops the warm loop.
func (h *EagerHandler) Close() error {
	if h.closed {
		return nil
	}
	h.closed = true
	h.commands <- eagerClose
	return nil
}

// ShouldWarm returns warm signals for the attached target.
func (h *EagerHandler) ShouldWarm() <-chan struct{} {
	return h.ch
}

func (h *EagerHandler) loop() {
	ticker := time.NewTicker(eagerWarmInterval)
	ticker.Stop()
	defer ticker.Stop()

	running := false
	for {
		select {
		case <-ticker.C:
			if running {
				select {
				case h.ch <- struct{}{}:
				default:
				}
			}
		case cmd := <-h.commands:
			switch cmd {
			case eagerStart:
				if !running {
					ticker.Reset(eagerWarmInterval)
					running = true
				}
			case eagerStop:
				if running {
					ticker.Stop()
					running = false
				}
			case eagerClose:
				return
			}
		}
	}
}

// NewEagerHandler creates an eager frontend handler.
func NewEagerHandler(_ string, _ config.Protocol, _ netip.AddrPort, _ map[string]any) (frontend.Handler, error) {
	h := &EagerHandler{
		ch:       make(chan struct{}, 1),
		commands: make(chan eagerCommand),
	}
	go h.loop()
	return h, nil
}
