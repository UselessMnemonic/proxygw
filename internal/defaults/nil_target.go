package defaults

import "proxygw/internal/target"

var NilTarget target.Kind = nilTarget{}

type nilTarget struct{}

func (nilTarget) Name() string {
	return "nil"
}

func (nilTarget) New(map[string]any) (target.Driver, error) {
	return nilTarget{}, nil
}

func (nilTarget) Kind() target.Kind {
	return NilTarget
}

func (nilTarget) Warm() error {
	return nil
}

func (nilTarget) Drain() error {
	return nil
}

func (nilTarget) Close() {}
