package targets

import (
	"github.com/UselessMnemonic/proxygw/pkg/target"
)

// NoneHandler is a no-op target driver useful for tests and static endpoints.
type NoneHandler struct{}

// Warm accepts the lifecycle transition without doing work.
func (NoneHandler) Warm() error {
	return nil
}

// Drain accepts the lifecycle transition without doing work.
func (NoneHandler) Drain() error {
	return nil
}

// Close accepts the lifecycle transition without doing work.
func (NoneHandler) Close() error {
	return nil
}

// NewNoneHandler creates a no-op target handler.
func NewNoneHandler(name string, options map[string]any) (target.Handler, error) {
	return NoneHandler{}, nil
}
