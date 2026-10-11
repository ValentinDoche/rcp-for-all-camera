package core

import "context"

// ConnState is a camera connection state.
type ConnState string

const (
	StateDisconnected ConnState = "disconnected"
	StateConnecting   ConnState = "connecting"
	StateConnected    ConnState = "connected"
	StateError        ConnState = "error"
)

// Status is a snapshot of a camera's connection.
type Status struct {
	State ConnState `json:"state"`
	// Detail is a short human-readable reason for StateError. It must not
	// contain credentials.
	Detail string `json:"detail,omitempty"`
}

// Event is pushed to subscribers. Exactly one of these is set:
//   - Status, when the connection state changed;
//   - Control and Value, when a control changed (on the camera or by Set).
type Event struct {
	Status  *Status   `json:"status,omitempty"`
	Control ControlID `json:"control,omitempty"`
	Value   Value     `json:"value"`
}

// Camera is one configured camera. Implementations live in
// internal/cameras/<driver> and must be safe for concurrent use.
//
// Contract:
//   - Controls is static for a given model and config; it never needs a
//     connection, so the GUI can lay out controls for an offline camera.
//   - Set validates v against the control's spec before any I/O and returns
//     ErrUnknownControl, ErrInvalidValue or ErrReadOnly without touching the
//     network. A command that fails on the wire returns an error; it is never
//     silently dropped.
//   - Connect returns once the first connection attempt is done. The driver
//     then keeps reconnecting by itself until Close.
//   - Subscribe callbacks run on a driver goroutine and must not block.
type Camera interface {
	Controls() []ControlSpec
	Connect(ctx context.Context) error
	Close() error
	Status() Status
	Get(ctx context.Context, id ControlID) (Value, error)
	Set(ctx context.Context, id ControlID, v Value) error
	Subscribe(fn func(Event)) (cancel func())
}

// Driver builds cameras for the models it declares. One driver per camera
// protocol family (blackmagic, sony, panasonic...).
type Driver interface {
	// Name is the driver id, also the prefix of its model ids ("sony").
	Name() string
	Models() []ModelInfo
	// New builds a camera for an already validated config. It must not do
	// any I/O; connecting is Camera.Connect's job.
	New(m ModelInfo, cfg CameraConfig) (Camera, error)
}
