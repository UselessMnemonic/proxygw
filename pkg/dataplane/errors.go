package dataplane

import "errors"

var ErrClosed error = errors.New("closed")

var ErrGroupAlreadyRegistered = errors.New("group already registered")

var ErrNoSuchMapping = errors.New("no mapping found")
