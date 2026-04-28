package engine

import (
	"errors"
)

// ErrClosed is returned when the engine has begun shutdown.
var ErrClosed = errors.New("engine closed")

// ErrTargetKindAlreadyRegistered is returned when a target kind name is reused.
var ErrTargetKindAlreadyRegistered = errors.New("target kind already registered")

// ErrTargetKindNotRegistered is returned when a target kind has not been registered.
var ErrTargetKindNotRegistered = errors.New("target kind not registered")

// ErrTargetAlreadyRegistered is returned when a target name is reused.
var ErrTargetAlreadyRegistered = errors.New("target already registered")

// ErrTargetNotRegistered is returned when a target lookup fails.
var ErrTargetNotRegistered = errors.New("target not registered")

// ErrTargetInUse is returned when deleting a target that is still live.
var ErrTargetInUse = errors.New("target in use")

// ErrFrontendKindAlreadyRegistered is returned when a frontend kind name is reused.
var ErrFrontendKindAlreadyRegistered = errors.New("frontend kind already registered")

// ErrFrontendKindNotRegistered is returned when a frontend kind has not been registered.
var ErrFrontendKindNotRegistered = errors.New("frontend kind not registered")

// ErrFrontendAlreadyRegistered is returned when a frontend name is reused.
var ErrFrontendAlreadyRegistered = errors.New("frontend already registered")

// ErrFrontendInUse is returned when deleting a frontend that is still live.
var ErrFrontendInUse = errors.New("frontend in use")
