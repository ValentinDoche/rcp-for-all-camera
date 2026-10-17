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
	// Detail is a short human-readable reason for StateError, or for
	// StateDisconnected while the driver retries. It must not contain
	// credentials.
	Detail string `json:"detail,omitempty"`
}

// Event is pushed to subscribers. Exactly one of these is set:
//   - Status, when the connection state changed;
//   - Control and Value, when a control changed (on the camera or by Set);
//   - Lens, when the lens information changed (LensReporter cameras only).
type Event struct {
	Status  *Status   `json:"status,omitempty"`
	Control ControlID `json:"control,omitempty"`
	Value   Value     `json:"value"`
	Lens    *Lens     `json:"lens,omitempty"`
}

// Camera is one configured camera. The application gets them from the
// camera modules (internal/plugins); implementations must be safe for
// concurrent use.
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
//     A driver never calls them from inside Subscribe itself (the app may
//     hold its lock while subscribing).
type Camera interface {
	Controls() []ControlSpec
	Connect(ctx context.Context) error
	Close() error
	Status() Status
	Get(ctx context.Context, id ControlID) (Value, error)
	Set(ctx context.Context, id ControlID, v Value) error
	Subscribe(fn func(Event)) (cancel func())
}

// Module is a camera module: the code that controls one camera model.
// internal/plugins turns each WebAssembly file of the modules folder into
// a Module.
type Module interface {
	// Info describes the model. It is static: the registry reads it once.
	Info() ModelInfo
	// New builds a camera for a config the registry already validated
	// (Model is Info().ID, Port is filled). It must not do any I/O;
	// connecting is Camera.Connect's job.
	New(cfg CameraConfig) (Camera, error)
}
