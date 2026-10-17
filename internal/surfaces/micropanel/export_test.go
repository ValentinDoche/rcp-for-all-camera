package micropanel

// Internals the external tests check: the auth algorithm, the report
// parser and the seven-segment font.
const (
	RepKeys     = repKeys
	RepAnalog   = repAnalog
	RepJoystick = repJoystick
	SegDP       = segDP
)

var (
	AuthResponse = authResponse
	SegFont      = segFont
)

// ParseReport applies one input report to s.
func (s *State) ParseReport(p []byte) bool { return s.parseReport(p) }
