package frontend

// Driver manages the lifecycle of a concrete frontend listener.
type Driver interface {
	Kind() Kind
	Start() error
	Stop() error
	ShouldWarm() <-chan struct{}
	Close()
}
