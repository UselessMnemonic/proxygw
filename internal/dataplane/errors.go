package dataplane

import "errors"

var ErrClosed = errors.New("dataplane closed")

var ErrGroupAlreadyRegistered = errors.New("group already registered")
