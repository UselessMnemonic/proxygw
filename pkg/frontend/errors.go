package frontend

import "errors"

// ErrClosed means a frontend has been closed.
var ErrClosed = errors.New("closed")
