package frontend

type Driver interface {
	Kind() Kind
	Start() error
	Stop() error
	Close()
}
