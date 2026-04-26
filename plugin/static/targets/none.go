package targets

import (
	"proxygw/internal/target"
)

type NoneHandler struct{}

func (NoneHandler) Warm() error {
	return nil
}

func (NoneHandler) Drain() error {
	return nil
}

func (NoneHandler) Close() error {
	return nil
}

func NewNoneHandler(name string, options map[string]any) (target.Handler, error) {
	return NoneHandler{}, nil
}
