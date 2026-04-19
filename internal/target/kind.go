package target

type Kind interface {
	Name() string
	New(map[string]any) (Driver, error)
}
