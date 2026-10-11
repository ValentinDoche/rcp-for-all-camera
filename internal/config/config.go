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
func (s FileStore) Load() (Rig, error) {
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
		return empty, fmt.Errorf("config file larger than %d bytes", MaxFileSize)
	}
	var rig Rig
	if err := json.Unmarshal(b, &rig); err != nil {
		return empty, fmt.Errorf("parse config: %w", err)
	}
	if rig.Version != Version {
		return empty, fmt.Errorf("version %d: %w", rig.Version, ErrUnsupportedVersion)
	}
	if rig.Active < 0 || rig.Active > Slots {
		rig.Active = 0
	}
	return rig, nil
}

// Save writes the rig atomically (temp file + rename), readable only by the
// current user.
func (s FileStore) Save(rig Rig) error {
	rig.Version = Version
	b, err := json.MarshalIndent(rig, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".rig-*.json")
	if err != nil {
		return fmt.Errorf("create temp config: %w", err)
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return fmt.Errorf("write config: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close config: %w", err)
	}
	if err := os.Rename(tmp.Name(), s.Path); err != nil {
		return fmt.Errorf("replace config: %w", err)
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
