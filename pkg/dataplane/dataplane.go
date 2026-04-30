package dataplane

// Dataplane is an abstraction of the system's network stack.
// Any dataplane must be capable of DNAT and flow tracking.
type Dataplane interface {
	// Name retrieves the name of the dataplane.
	Name() string
	// NewGroup registers a new valid Group to the underlying dataplane.
	// Returns ErrClosed or ErrGroupAlreadyRegistered.
	NewGroup(name string) (Group, error)
	// StaleGroups is a potentially long, blocking operation that retrieves
	// the Groups known to be stale from since a given time.
	// A Group is considered stale when there is no traffic known to
	// the dataplane to any Mapping.Destination. Returns ErrClosed.
	StaleGroups() ([]Group, error)
	// Close invalidates all groups and closes all held resources.
	// When closed, all operations fail with ErrClosed.
	Close() error
}
