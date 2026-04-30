package plugin

import (
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

var registry = make(map[string]Handler)

// Register adds a plugin handler to the process-local registry.
func Register(source string, handler Handler) bool {
	if source == "" {
		return false
	}
	if _, exists := registry[source]; exists {
		return false
	}
	registry[source] = handler
	return true
}

func Find(source string) (Handler, bool) {
	handler, exists := registry[source]
	return handler, exists
}
