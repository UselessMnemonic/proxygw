package target

import "errors"

// ErrClosed means a target has been closed.
var ErrClosed = errors.New("target closed")
