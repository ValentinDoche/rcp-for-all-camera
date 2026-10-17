package core

import "errors"

// Sentinel errors. Callers test them with errors.Is; drivers wrap them with
// fmt.Errorf("...: %w", ErrX) to add context.
var (
	ErrNotImplemented = errors.New("not implemented")
	ErrUnknownModel   = errors.New("unknown camera model")
	ErrUnknownControl = errors.New("unknown control")
	ErrInvalidValue   = errors.New("invalid value")
	ErrReadOnly       = errors.New("control is read-only")
	ErrNotConnected   = errors.New("camera not connected")
	ErrInvalidConfig  = errors.New("invalid camera config")
	ErrDuplicateModel = errors.New("model already registered")
)
