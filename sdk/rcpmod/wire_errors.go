package rcpmod

import "errors"

// The errors a camera returns. The application maps them to the codes the
// GUI shows; any other error becomes an internal error. Wrap them with
// fmt.Errorf("...: %w", ErrX) to add context.
var (
	ErrUnknownControl = errors.New("unknown control")
	ErrInvalidValue   = errors.New("invalid value")
	ErrReadOnly       = errors.New("control is read-only")
	ErrNotConnected   = errors.New("camera not connected")
	ErrNotImplemented = errors.New("not implemented")
)

// Wire error codes, one per error above, plus CodeInternal.
const (
	CodeUnknownControl = "UNKNOWN_CONTROL"
	CodeInvalidValue   = "INVALID_VALUE"
	CodeReadOnly       = "READ_ONLY"
	CodeNotConnected   = "NOT_CONNECTED"
	CodeNotImplemented = "NOT_IMPLEMENTED"
	CodeInternal       = "INTERNAL"
)

var codes = []struct {
	err  error
	code string
}{
	{ErrUnknownControl, CodeUnknownControl},
	{ErrInvalidValue, CodeInvalidValue},
	{ErrReadOnly, CodeReadOnly},
	{ErrNotConnected, CodeNotConnected},
	{ErrNotImplemented, CodeNotImplemented},
}

// CodeOf returns the wire code of err.
func CodeOf(err error) string {
	for _, c := range codes {
		if errors.Is(err, c.err) {
			return c.code
		}
	}
	return CodeInternal
}

// ErrorOf returns the error a wire code stands for (nil for CodeInternal
// or an unknown code).
func ErrorOf(code string) error {
	for _, c := range codes {
		if c.code == code {
			return c.err
		}
	}
	return nil
}
