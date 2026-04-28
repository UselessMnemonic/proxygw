package dataplane

import "errors"

// ErrClosed is returned when a caller tries to mutate a closed dataplane.
var ErrClosed = errors.New("dataplane closed")

// ErrGroupAlreadyRegistered is returned when a DNAT group name is reused.
var ErrGroupAlreadyRegistered = errors.New("group already registered")
