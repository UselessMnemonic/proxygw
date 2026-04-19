package frontend

// State describes listener health and lifecycle.
type State int32

const (
	// Stopped indicates the frontend is not accepting traffic.
	Stopped State = 1
	// Starting indicates the frontend is preparing to accept traffic
	Starting State = 2
	// Running indicates the frontend is actively listening.
	Running State = 3
	// Stopping indicates the frontend is draining traffic
	Stopping State = 4
	// Closed indicates the frontend is no longer valid.
	Closed State = 5
)

func (it State) String() string {
	switch it {
	case Stopped:
		return "stopped"
	case Starting:
		return "starting"
	case Running:
		return "running"
	case Stopping:
		return "stopping"
	case Closed:
		return "closed"
	default:
		return "invalid"
	}
}
