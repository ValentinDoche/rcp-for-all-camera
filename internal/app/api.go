// Package app wires the modules together and exposes Service, the API the
// GUI calls directly; docs/app-api.md is the reference.
package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// SlotCount is the number of camera buttons on the panel.
const SlotCount = 8

// Event names pushed to the GUI through the Emitter.
const (
	EventPanelStatus = "panel:status" // payload PanelStatus
	EventRigChanged  = "rig:changed"  // payload Rig
	EventCamera      = "camera:event" // payload CameraEvent
)

// PanelStatus is the USB panel detection state.
type PanelStatus struct {
	Connected bool   `json:"connected"`
	Serial    string `json:"serial,omitempty"`
	Detail    string `json:"detail,omitempty"`
}

// Rig is the whole camera assignment: always SlotCount slots, numbered 1..8.
type Rig struct {
	Slots  []Slot `json:"slots"`
	Active int    `json:"active"` // selected slot, 0 = none
}

// Slot is one camera button. Camera is nil when the slot is empty.
type Slot struct {
	Slot   int         `json:"slot"`
	Camera *CameraView `json:"camera"`
}

// CameraView is a configured camera as the GUI sees it. The password is
// never sent back; HasPassword says whether one is stored.
type CameraView struct {
	Model       core.ModelID      `json:"model"`
	Brand       string            `json:"brand"`
	ModelName   string            `json:"modelName"`
	Support     core.SupportLevel `json:"support"`
	Name        string            `json:"name"`
	Host        string            `json:"host"`
	Port        int               `json:"port"`
	Transport   core.Transport    `json:"transport"`
	Username    string            `json:"username,omitempty"`
	HasPassword bool              `json:"hasPassword"`
	Status      core.Status       `json:"status"`
}

// SlotInput is what the GUI sends to assign or edit a slot.
type SlotInput struct {
	Model     core.ModelID   `json:"model"`
	Name      string         `json:"name"`
	Host      string         `json:"host"`
	Port      int            `json:"port"` // 0 = model default
	Transport core.Transport `json:"transport"`
	Username  string         `json:"username"`
	// Password: null keeps the stored one, "" clears it, any other value
	// replaces it.
	Password *string `json:"password"`
}

// String, GoString and LogValue keep the password out of fmt verbs and slog.
func (in SlotInput) String() string {
	return fmt.Sprintf("{Model:%s Name:%s Host:%s Port:%d Transport:%s Username:%s Password:%s}",
		in.Model, in.Name, in.Host, in.Port, in.Transport, in.Username, in.passwordState())
}

func (in SlotInput) GoString() string { return "app.SlotInput" + in.String() }

func (in SlotInput) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("model", string(in.Model)), slog.String("name", in.Name),
		slog.String("host", in.Host), slog.Int("port", in.Port),
		slog.String("transport", string(in.Transport)), slog.String("username", in.Username),
		slog.String("password", in.passwordState()),
	)
}

func (in SlotInput) passwordState() string {
	switch {
	case in.Password == nil:
		return "keep"
	case *in.Password == "":
		return "clear"
	}
	return "[redacted]"
}

// ControlView is one control of a camera with its last known value.
// Value is nil while the value is unknown (camera offline).
type ControlView struct {
	Spec  core.ControlSpec `json:"spec"`
	Value *core.Value      `json:"value"`
}

// CameraEvent is a core.Event tagged with its slot.
type CameraEvent struct {
	Slot  int        `json:"slot"`
	Event core.Event `json:"event"`
}

// Error codes. They are part of the GUI contract: add, never rename.
const (
	CodeInvalidSlot    = "INVALID_SLOT"
	CodeEmptySlot      = "EMPTY_SLOT"
	CodeInvalidConfig  = "INVALID_CONFIG"
	CodeUnknownModel   = "UNKNOWN_MODEL"
	CodeUnknownControl = "UNKNOWN_CONTROL"
	CodeInvalidValue   = "INVALID_VALUE"
	CodeReadOnly       = "READ_ONLY"
	CodeNotConnected   = "NOT_CONNECTED"
	CodeNotImplemented = "NOT_IMPLEMENTED"
	CodeInternal       = "INTERNAL"
)

// Error is the only error type Service returns. Error() is the JSON
// {"code": "...", "message": "..."}, a stable form for logs and remote callers.
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	err     error
}

func (e *Error) Error() string {
	b, _ := json.Marshal(e)
	return string(b)
}

func (e *Error) Unwrap() error { return e.err }

// toAPIError maps internal errors to API errors. Unknown errors become
// INTERNAL with a generic message so internals never leak to the GUI.
func toAPIError(err error) error {
	if err == nil {
		return nil
	}
	var ae *Error
	if errors.As(err, &ae) {
		return ae
	}
	for _, m := range []struct {
		target error
		code   string
	}{
		{core.ErrInvalidConfig, CodeInvalidConfig},
		{core.ErrUnknownModel, CodeUnknownModel},
		{core.ErrUnknownControl, CodeUnknownControl},
		{core.ErrInvalidValue, CodeInvalidValue},
		{core.ErrReadOnly, CodeReadOnly},
		{core.ErrNotConnected, CodeNotConnected},
		{core.ErrNotImplemented, CodeNotImplemented},
	} {
		if errors.Is(err, m.target) {
			return &Error{Code: m.code, Message: err.Error(), err: err}
		}
	}
	return &Error{Code: CodeInternal, Message: "internal error", err: err}
}
