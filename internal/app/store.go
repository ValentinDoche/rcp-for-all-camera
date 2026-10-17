package app

import "github.com/valentindoche/rcp-for-all-camera/internal/core"

// The ports through which the Service persists its state. The adapters
// (internal/config) decide the storage format; the Service only sees these
// types.

// SavedRig is what the Service saves: the cameras, the settings and the
// presets. Passwords are never in it: they go to Secrets.
type SavedRig struct {
	// Slots[i] is camera button i+1; nil = empty. Entries are checked
	// against the model registry at load, and dropped if invalid.
	Slots  [SlotCount]*core.CameraConfig
	Active int
	// BindIP is the local address used to reach cameras ("" = OS default).
	BindIP string
	// OperatorLock: nil means on (the default).
	OperatorLock *bool
	// Panel is the panel source: "" or "usb" = the first USB panel,
	// "usb:<serial>" = that panel, "sim" = the simulator.
	Panel string
	// LogLevel is LogNormal ("") or LogDebug.
	LogLevel string
	Presets  []SavedPreset
}

// SavedPreset is a snapshot of one camera's control values. Model is the
// camera model it was taken on, so it is never applied to another model.
type SavedPreset struct {
	Slot   int // 1..SlotCount
	Number int // 1..PresetCount
	Model  core.ModelID
	Values map[core.ControlID]core.Value
}

// Store loads and saves the rig (config.FileStore in production). Load
// returns an empty rig when nothing was saved yet.
type Store interface {
	Load() (SavedRig, error)
	Save(SavedRig) error
}

// Secrets stores camera passwords outside the rig.
type Secrets interface {
	Get(key string) (value string, ok bool, err error)
	Set(key, value string) error
	Delete(key string) error // idempotent
}
