package plugin

import (
	internal "github.com/UselessMnemonic/proxygw/internal/plugin"
)

// Handler defines plugin lifecycle hooks.
type Handler = internal.Handler

// Namespace contains logically grouped resources for a plugin
type Namespace = internal.Namespace

// Register registers a plugin.
func Register(name string, handler Handler) error {
	return internal.Register(name, handler)
}
