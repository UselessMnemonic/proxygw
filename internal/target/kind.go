package target

// Kind identifies a target implementation and constructs drivers for named targets.
type Kind interface {
	Name() string
	New(name string, options map[string]any) (Driver, error)
}
