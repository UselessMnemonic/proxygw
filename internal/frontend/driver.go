package frontend

type Driver interface {
	Kind() Kind
	Start() error
	Stop() error
	ShouldWarm() <-chan struct{}
	Close()
}
