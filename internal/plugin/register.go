package plugin

import (
	"fmt"
	"proxygw/internal/engine"
	"proxygw/internal/frontend"
	"proxygw/internal/target"
	"sync"
)

type Namespace struct {
	Frontends map[string]frontend.HandlerCtor
	Targets   map[string]target.HandlerCtor
}

type Handler struct {
	OnLoad   func(config map[string]any, engine *engine.Engine, namespace *Namespace) error
	OnUnload func() error
}

func Register(name string, handler Handler) error {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if _, exists := registry.handlers[name]; exists {
		return fmt.Errorf("%q is already registered", name)
	}
	registry.handlers[name] = handler
	return nil
}

func Export() map[string]Handler {
	return registry.handlers
}

var registry = struct {
	mu       sync.Mutex
	handlers map[string]Handler
}{
	handlers: make(map[string]Handler),
}
