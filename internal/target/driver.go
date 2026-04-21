package target

// Driver manages the lifecycle of a concrete target instance.
type Driver interface {
	Kind() Kind
	Warm() error
	Drain() error
	Close()
}
