package target

type Driver interface {
	Kind() Kind
	Warm() error
	Drain() error
	Close()
}
