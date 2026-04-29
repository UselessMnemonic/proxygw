package frontends

import (
	"net/netip"
	"time"

	"github.com/UselessMnemonic/proxygw/pkg/config"
	"github.com/UselessMnemonic/proxygw/pkg/frontend"
)

const alwaysWarmInterval = time.Second

type alwaysCommand uint8

const (
	alwaysStart alwaysCommand = iota
	alwaysStop
	alwaysClose
)

// AlwaysHandler is a frontend handler that periodically asks its target to warm
// while the frontend is running.
type AlwaysHandler struct {
	ch       chan struct{}
	commands chan alwaysCommand
	closed   bool
}

// Start begins emitting warm signals.
func (h *AlwaysHandler) Start() error {
	if h.closed {
		return nil
	}
	h.commands <- alwaysStart
	return nil
}

// Stop pauses warm signals.
func (h *AlwaysHandler) Stop() error {
	if h.closed {
		return nil
	}
	h.commands <- alwaysStop
	return nil
}

// Close permanently stops the warm loop.
func (h *AlwaysHandler) Close() error {
	if h.closed {
		return nil
	}
	h.closed = true
	h.commands <- alwaysClose
	return nil
}

// ShouldWarm returns warm signals for the attached target.
func (h *AlwaysHandler) ShouldWarm() <-chan struct{} {
	return h.ch
}

func (h *AlwaysHandler) loop() {
	ticker := time.NewTicker(alwaysWarmInterval)
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
			case alwaysStart:
				if !running {
					ticker.Reset(alwaysWarmInterval)
					running = true
				}
			case alwaysStop:
				if running {
					ticker.Stop()
					running = false
				}
			case alwaysClose:
				return
			}
		}
	}
}

// NewAlwaysHandler creates an always frontend handler.
func NewAlwaysHandler(_ string, _ config.Protocol, _ netip.AddrPort, _ map[string]any) (frontend.Handler, error) {
	h := &AlwaysHandler{
		ch:       make(chan struct{}, 1),
		commands: make(chan alwaysCommand),
	}
	go h.loop()
	return h, nil
}
