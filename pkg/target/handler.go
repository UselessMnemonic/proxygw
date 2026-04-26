package target

// Handler manages the lifecycle of a target instance.
type Handler interface {
	// Warm invokes the handler to become available, blocking until ready.
	Warm() error
	// Drain invokes the handler to tear down expensive resources, blocking until done.
	Drain() error
	// Close tears down the handler, blocking until done.
	Close() error
}

// HandlerCtor is a function that creates a new target.
type HandlerCtor func(name string, options map[string]any) (Handler, error)
