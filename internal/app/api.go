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

// PresetCount is the number of presets per slot (the panel's scene buttons).
const PresetCount = 5

// Event names pushed to the GUI through the Emitter.
const (
	EventPanelStatus = "panel:status" // payload PanelStatus
	EventRigChanged  = "rig:changed"  // payload Rig
	EventCamera      = "camera:event" // payload CameraEvent
	// EventCommandFailed: a control write (from any surface or a preset
	// recall) failed. Payload CommandFailed. The panel flashes ALARM.
	EventCommandFailed = "camera:commandFailed"
	// EventStoreArmed: the shared "Store" arming changed (the panel's STORE
	// key and the GUI's Store button). Payload StoreArmedState. Emitted
	// only on a real change.
	EventStoreArmed = "preset:storeArmed"
)

// StoreArmedState is the payload of EventStoreArmed.
type StoreArmedState struct {
	Armed bool `json:"armed"`
}

// CommandFailed tells which write failed, with its error code.
type CommandFailed struct {
	Slot    int            `json:"slot"`
	Control core.ControlID `json:"control"`
	Code    string         `json:"code"`
}

// PanelStatus is the USB panel state. Connected = plugged in; Active = opened,
// authenticated and driven by the app. Connected && !Active with a Detail
// means the panel is there but cannot be driven (for example another
// program holds it).
type PanelStatus struct {
	Connected bool   `json:"connected"`
	Active    bool   `json:"active"`
	Serial    string `json:"serial,omitempty"`
	Detail    string `json:"detail,omitempty"`
}

// Rig is the whole camera assignment: always SlotCount slots, numbered 1..8.
type Rig struct {
	Slots  []Slot `json:"slots"`
	Active int    `json:"active"` // selected slot, 0 = none
}

// Slot is one camera button. Camera is nil when the slot is empty.
// Presets lists the stored preset numbers (1..PresetCount), sorted; Rig()
// never leaves it nil, so it is [] in JSON when there is none.
type Slot struct {
	Slot    int         `json:"slot"`
	Camera  *CameraView `json:"camera"`
	Presets []int       `json:"presets"`
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
	Username    string            `json:"username,omitempty"`
	HasPassword bool              `json:"hasPassword"`
	// PowerZoom: the lens has a motorised zoom (see SlotInput.PowerZoom).
	PowerZoom bool `json:"powerZoom"`
	// InterchangeableLens: the model takes any lens, so PowerZoom is
	// editable; otherwise its zoom is built in.
	InterchangeableLens bool        `json:"interchangeableLens"`
	Status              core.Status `json:"status"`
}

// SlotInput is what the GUI sends to assign or edit a slot.
type SlotInput struct {
	Model    core.ModelID `json:"model"`
	Name     string       `json:"name"`
	Host     string       `json:"host"`
	Port     int          `json:"port"` // 0 = model default
	Username string       `json:"username"`
	// Password: null keeps the stored one, "" clears it, any other value
	// replaces it.
	Password *string `json:"password"`
	// PowerZoom: the mounted lens has a motorised zoom. Only used by models
	// with an interchangeable lens; without it the camera has no zoom
	// control.
	PowerZoom bool `json:"powerZoom"`
}

// String, GoString and LogValue keep the password out of fmt verbs and slog.
func (in SlotInput) String() string {
	return fmt.Sprintf("{Model:%s Name:%s Host:%s Port:%d Username:%s Password:%s PowerZoom:%t}",
		in.Model, in.Name, in.Host, in.Port, in.Username, in.passwordState(), in.PowerZoom)
}

func (in SlotInput) GoString() string { return "app.SlotInput" + in.String() }

func (in SlotInput) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("model", string(in.Model)), slog.String("name", in.Name),
		slog.String("host", in.Host), slog.Int("port", in.Port),
		slog.String("username", in.Username),
		slog.String("password", in.passwordState()),
		slog.Bool("powerZoom", in.PowerZoom),
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
	// CodeConfirmRequired: a guarded control (record, color bars...) was
	// written while the operator lock is on; ask the user, then call
	// ConfirmSetControl.
	CodeConfirmRequired = "CONFIRM_REQUIRED"
	// CodeEmptyPreset: no preset is stored under that number.
	CodeEmptyPreset = "EMPTY_PRESET"
	// CodeInvalidPreset: the preset number is out of range, or the preset
	// was taken on another camera model.
	CodeInvalidPreset = "INVALID_PRESET"
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
