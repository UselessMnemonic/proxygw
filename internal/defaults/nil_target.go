package defaults

import "proxygw/internal/target"

type NilTarget struct{}

func (NilTarget) Name() string {
	return "nil"
}

func (NilTarget) New(map[string]any) (target.Driver, error) {
	return NilTarget{}, nil
}

func (NilTarget) Kind() target.Kind {
	return NilTarget{}
}

func (NilTarget) Warm() error {
	return nil
}

func (NilTarget) Drain() error {
	return nil
}

func (NilTarget) Close() {}
