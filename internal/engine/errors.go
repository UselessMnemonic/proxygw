package engine

import (
	"errors"
)

var ErrClosed = errors.New("engine closed")

var ErrTargetKindAlreadyRegistered = errors.New("target kind already registered")

var ErrTargetKindNotRegistered = errors.New("target kind not registered")

var ErrTargetAlreadyRegistered = errors.New("target already registered")

var ErrTargetNotRegistered = errors.New("target not registered")

var ErrTargetInUse = errors.New("target in use")

var ErrFrontendKindAlreadyRegistered = errors.New("frontend kind already registered")

var ErrFrontendKindNotRegistered = errors.New("frontend kind not registered")

var ErrFrontendAlreadyRegistered = errors.New("frontend already registered")

var ErrFrontendInUse = errors.New("frontend in use")
