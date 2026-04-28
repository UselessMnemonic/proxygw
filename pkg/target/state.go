package target

// State describes the target lifecycle.
type State int32

const (
	// Inactive means the target is not accepting traffic.
	Inactive State = 1
	// Active means the target is actively accepting traffic.
	Active State = 2
	// Warming means the target is being prepared to accept traffic.
	Warming State = 3
	// Draining means the target is being taken down to stop accepting traffic.
	Draining State = 4
	// Closed means the target is no longer valid.
	Closed State = 6
)

// String returns the stable lowercase name used in logs and status responses.
func (it State) String() string {
	switch it {
	case Inactive:
		return "inactive"
	case Active:
		return "active"
	case Warming:
		return "warming"
	case Draining:
		return "draining"
	case Closed:
		return "closed"
	default:
		return "invalid"
	}
}
