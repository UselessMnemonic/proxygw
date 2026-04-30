package plugin

import (
	internal "github.com/UselessMnemonic/proxygw/internal/plugin"
)

// Handler defines plugin lifecycle hooks.
type Handler = internal.Handler

// Namespace contains logically grouped resources for a plugin
type Namespace = internal.Namespace

// Register registers a plugin by its source import path.
func Register(source string, handler Handler) bool {
	return internal.Register(source, handler)
}
