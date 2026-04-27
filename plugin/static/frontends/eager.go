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

type EagerHandler struct {
	ch       chan struct{}
	commands chan eagerCommand
	closed   bool
}

func (h *EagerHandler) Start() error {
	if h.closed {
		return nil
	}
	h.commands <- eagerStart
	return nil
}

func (h *EagerHandler) Stop() error {
	if h.closed {
		return nil
	}
	h.commands <- eagerStop
	return nil
}

func (h *EagerHandler) Close() error {
	if h.closed {
		return nil
	}
	h.closed = true
	h.commands <- eagerClose
	return nil
}

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

func NewEagerHandler(_ string, _ config.Protocol, _ netip.AddrPort, _ map[string]any) (frontend.Handler, error) {
	h := &EagerHandler{
		ch:       make(chan struct{}, 1),
		commands: make(chan eagerCommand),
	}
	go h.loop()
	return h, nil
}
