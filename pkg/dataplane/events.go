package dataplane

import "time"

// DNATGroupTimeoutEvent is emitted when a group appears idle long enough to be
// considered for draining.
type DNATGroupTimeoutEvent struct {
	// Timestamp records when the timeout was detected.
	Timestamp time.Time
}
