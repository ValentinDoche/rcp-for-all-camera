// Package config persists the rig (camera assignments) and the camera
// passwords. Passwords never go into the rig file: they live in a Secrets
// store (OS-protected on Windows).
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// Version is the current rig file format version.
const Version = 1

// Slots is the number of camera slots in a rig file.
const Slots = 8

// MaxFileSize bounds what Load will read, so a corrupt or hostile file
// cannot exhaust memory.
const MaxFileSize = 1 << 20

var ErrUnsupportedVersion = errors.New("unsupported config version")

// Rig is the on-disk rig. Slots[i] is camera button i+1; nil = empty.
// Entries are not validated here: the app checks them against the model
// registry and drops the invalid ones.
type Rig struct {
	Version int                       `json:"version"`
	Slots   [Slots]*core.CameraConfig `json:"slots"`
	Active  int                       `json:"active"`
	// BindIP is the local address used to reach cameras ("" = OS default).
	BindIP string `json:"bindIp,omitempty"`
	// OperatorLock: nil means on (the default).
	OperatorLock *bool `json:"operatorLock,omitempty"`
	// Panel is the panel source chosen in the app: "" or "usb" = the first
	// USB panel, "usb:<serial>" = that panel, "sim" = the simulator.
	Panel string `json:"panel,omitempty"`
	// LogLevel is "normal" (default) or "debug".
	LogLevel string `json:"logLevel,omitempty"`
	// Presets are the stored camera presets. They live here, never on the
	// camera.
	Presets []Preset `json:"presets,omitempty"`
}

// Preset is a snapshot of one camera's control values, stored by the app and
// recalled by writing the values back. Model is the camera model it was taken
// on, so it is never applied to another model.
type Preset struct {
	Slot   int                           `json:"slot"`   // 1..Slots
	Number int                           `json:"number"` // 1..n
	Model  core.ModelID                  `json:"model"`
	Values map[core.ControlID]core.Value `json:"values"`
}

// FileStore reads and writes a rig as JSON at Path.
type FileStore struct {
	Path string
}

// DefaultPath is %APPDATA%\RCPForAllCamera\rig.json (or the OS equivalent).
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("config dir: %w", err)
	}
	return filepath.Join(dir, "RCPForAllCamera", "rig.json"), nil
}

// Load reads the rig. A missing file is an empty rig, not an error.
//
// A file that exists but cannot be used (too large, malformed, unsupported
// version) is renamed to <path>.bad before the error is returned: the app
// then starts with an empty rig, and its next Save must not destroy the
// user's file. Unknown JSON fields are ignored for forward compatibility (a
// stray "password" key is never read: CameraConfig.Password is json:"-").
func (s FileStore) Load() (Rig, error) {
	rig, err := s.load()
	if errors.Is(err, errUnusable) {
		if rerr := os.Rename(s.Path, s.Path+".bad"); rerr != nil {
			err = errors.Join(err, fmt.Errorf("keep unreadable config aside: %w", rerr))
		}
	}
	return rig, err
}

// errUnusable marks a rig file that was read but cannot be used.
var errUnusable = errors.New("unusable config file")

func (s FileStore) load() (Rig, error) {
	empty := Rig{Version: Version}
	f, err := os.Open(s.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return empty, nil
	}
	if err != nil {
		return empty, fmt.Errorf("open config: %w", err)
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, MaxFileSize+1))
	if err != nil {
		return empty, fmt.Errorf("read config: %w", err)
	}
	if len(b) > MaxFileSize {
		return empty, fmt.Errorf("config file larger than %d bytes: %w", MaxFileSize, errUnusable)
	}
	var rig Rig
	if err := json.Unmarshal(b, &rig); err != nil {
		return empty, fmt.Errorf("parse config: %w: %w", errUnusable, err)
	}
	if rig.Version != Version {
		return empty, fmt.Errorf("version %d: %w: %w", rig.Version, ErrUnsupportedVersion, errUnusable)
	}
	if rig.Active < 0 || rig.Active > Slots {
		rig.Active = 0
	}
	return rig, nil
}

// Save writes the rig atomically (temp file + rename). The 0700/0600 modes
// only matter on Unix; on Windows the file inherits the ACL of %APPDATA%,
// which is private to the user (plus SYSTEM and Administrators). See
// docs/security.md.
func (s FileStore) Save(rig Rig) error {
	rig.Version = Version
	b, err := json.MarshalIndent(rig, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	return writeFileAtomic(s.Path, ".rig-*.json", b)
}

// writeFileAtomic replaces path with b through a temp file in the same
// directory, so a crash never leaves a half-written file.
func writeFileAtomic(path, pattern string, b []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, pattern) // mode 0600
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename
	name := filepath.Base(path)
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return fmt.Errorf("write %s: %w", name, err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync %s: %w", name, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close %s: %w", name, err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("replace %s: %w", name, err)
	}
	return nil
}

// Secrets stores camera passwords outside the rig file.
type Secrets interface {
	Get(key string) (value string, ok bool, err error)
	Set(key, value string) error
	Delete(key string) error // idempotent
}

// MemorySecrets keeps secrets in memory only (tests, and the fallback when
// no OS store is available: passwords are then asked again after restart).
type MemorySecrets struct {
	mu sync.Mutex
	m  map[string]string
}

func NewMemorySecrets() *MemorySecrets { return &MemorySecrets{m: map[string]string{}} }

func (s *MemorySecrets) Get(key string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[key]
	return v, ok, nil
}

func (s *MemorySecrets) Set(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = value
	return nil
}

func (s *MemorySecrets) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, key)
	return nil
}
