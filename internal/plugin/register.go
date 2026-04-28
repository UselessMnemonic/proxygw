package plugin

import (
	"fmt"
	"sync"

	"github.com/UselessMnemonic/proxygw/pkg/engine"
	"github.com/UselessMnemonic/proxygw/pkg/frontend"
	"github.com/UselessMnemonic/proxygw/pkg/target"
)

// Namespace is the table a plugin fills with the kinds it exposes.
type Namespace struct {
	Frontends map[string]frontend.HandlerCtor
	Targets   map[string]target.HandlerCtor
}

// Handler defines the optional lifecycle hooks for a registered plugin.
type Handler struct {
	OnLoad   func(config map[string]any, engine *engine.Engine, namespace *Namespace) error
	OnUnload func() error
}

// Register adds a plugin handler to the process-local registry.
func Register(name string, handler Handler) error {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	if _, exists := registry.handlers[name]; exists {
		return fmt.Errorf("%q is already registered", name)
	}
	registry.handlers[name] = handler
	return nil
}

// Export returns the process-local plugin registry.
func Export() map[string]Handler {
	return registry.handlers
}

var registry = struct {
	mu       sync.Mutex
	handlers map[string]Handler
}{
	handlers: make(map[string]Handler),
}
