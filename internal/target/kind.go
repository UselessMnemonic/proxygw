package target

type Kind interface {
	Name() string
	New(string, map[string]any) (Driver, error)
}
