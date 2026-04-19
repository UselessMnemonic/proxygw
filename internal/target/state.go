package target

// State describes target activation lifecycle.
type State int32

const (
	// Inactive means the target is not accepting forwarded traffic.
	Inactive State = 1
	// Active means the target is actively serving forwarded traffic.
	Active State = 2
	// Warming means activation is in progress.
	Warming State = 3
	// Draining means deactivation is in progress.
	Draining State = 4
	// Closed means the target is no longer valid.
	Closed State = 6
)

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
