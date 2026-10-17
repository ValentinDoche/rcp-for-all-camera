// Package config persists the rig (camera assignments, settings, presets)
// and the camera passwords: it implements the app.Store and app.Secrets
// ports. Passwords never go into the rig file: they live in a secrets
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

	"github.com/valentindoche/rcp-for-all-camera/internal/app"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// Version is the current rig file format version.
const Version = 1

// MaxFileSize bounds what Load will read, so a corrupt or hostile file
// cannot exhaust memory.
const MaxFileSize = 1 << 20

// ErrUnsupportedVersion is returned for a rig file written by a newer
// version of the application.
var ErrUnsupportedVersion = errors.New("unsupported config version")

// rigFile is the on-disk format of the rig (rig.json). It is the
// persistence model: the mappers below convert it from and to the
// application's app.SavedRig, so the file format never depends on domain
// types. Unknown fields are ignored for forward compatibility.
type rigFile struct {
	Version      int                        `json:"version"`
	Slots        [app.SlotCount]*cameraFile `json:"slots"`
	Active       int                        `json:"active"`
	BindIP       string                     `json:"bindIp,omitempty"`
	OperatorLock *bool                      `json:"operatorLock,omitempty"`
	Panel        string                     `json:"panel,omitempty"`
	LogLevel     string                     `json:"logLevel,omitempty"`
	Presets      []presetFile               `json:"presets,omitempty"`
}

// cameraFile is one camera. There is no password field: a stray
// "password" key in a hand-edited file is never read.
type cameraFile struct {
	Model     string `json:"model"`
	Name      string `json:"name"`
	Host      string `json:"host"`
	Port      int    `json:"port,omitempty"`
	Username  string `json:"username,omitempty"`
	PowerZoom bool   `json:"powerZoom,omitempty"`
}

type presetFile struct {
	Slot   int                  `json:"slot"`
	Number int                  `json:"number"`
	Model  string               `json:"model"`
	Values map[string]valueFile `json:"values"`
}

type valueFile struct {
	Kind   string  `json:"kind"`
	Number float64 `json:"number,omitempty"`
	Choice string  `json:"choice,omitempty"`
	On     bool    `json:"on,omitempty"`
	Text   string  `json:"text,omitempty"`
}

func toFile(r app.SavedRig) rigFile {
	f := rigFile{Version: Version, Active: r.Active, BindIP: r.BindIP, OperatorLock: r.OperatorLock, Panel: r.Panel, LogLevel: r.LogLevel}
	for i, c := range r.Slots {
		if c != nil {
			f.Slots[i] = &cameraFile{Model: string(c.Model), Name: c.Name, Host: c.Host, Port: c.Port, Username: c.Username, PowerZoom: c.PowerZoom}
		}
	}
	for _, p := range r.Presets {
		pf := presetFile{Slot: p.Slot, Number: p.Number, Model: string(p.Model), Values: make(map[string]valueFile, len(p.Values))}
		for id, v := range p.Values {
			pf.Values[string(id)] = valueFile{Kind: string(v.Kind), Number: v.Number, Choice: v.Choice, On: v.On, Text: v.Text}
		}
		f.Presets = append(f.Presets, pf)
	}
	return f
}

// fromFile converts what was read. Entries are not validated here: the
// application checks them against the model registry and drops the
// invalid ones.
func fromFile(f rigFile) app.SavedRig {
	r := app.SavedRig{Active: f.Active, BindIP: f.BindIP, OperatorLock: f.OperatorLock, Panel: f.Panel, LogLevel: f.LogLevel}
	for i, c := range f.Slots {
		if c != nil {
			r.Slots[i] = &core.CameraConfig{Model: core.ModelID(c.Model), Name: c.Name, Host: c.Host, Port: c.Port, Username: c.Username, PowerZoom: c.PowerZoom}
		}
	}
	for _, p := range f.Presets {
		sp := app.SavedPreset{Slot: p.Slot, Number: p.Number, Model: core.ModelID(p.Model), Values: make(map[core.ControlID]core.Value, len(p.Values))}
		for id, v := range p.Values {
			sp.Values[core.ControlID(id)] = core.Value{Kind: core.ControlKind(v.Kind), Number: v.Number, Choice: v.Choice, On: v.On, Text: v.Text}
		}
		r.Presets = append(r.Presets, sp)
	}
	return r
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
// user's file.
func (s FileStore) Load() (app.SavedRig, error) {
	f, err := s.load()
	if errors.Is(err, errUnusable) {
		if rerr := os.Rename(s.Path, s.Path+".bad"); rerr != nil {
			err = errors.Join(err, fmt.Errorf("keep unreadable config aside: %w", rerr))
		}
	}
	return fromFile(f), err
}

// errUnusable marks a rig file that was read but cannot be used.
var errUnusable = errors.New("unusable config file")

func (s FileStore) load() (rigFile, error) {
	empty := rigFile{Version: Version}
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
	var rig rigFile
	if err := json.Unmarshal(b, &rig); err != nil {
		return empty, fmt.Errorf("parse config: %w: %w", errUnusable, err)
	}
	if rig.Version != Version {
		return empty, fmt.Errorf("version %d: %w: %w", rig.Version, ErrUnsupportedVersion, errUnusable)
	}
	if rig.Active < 0 || rig.Active > app.SlotCount {
		rig.Active = 0
	}
	return rig, nil
}

// Save writes the rig atomically (temp file + rename). The 0700/0600 modes
// only matter on Unix; on Windows the file inherits the ACL of %APPDATA%,
// which is private to the user (plus SYSTEM and Administrators). See
// docs/security.md.
func (s FileStore) Save(rig app.SavedRig) error {
	b, err := json.MarshalIndent(toFile(rig), "", "  ")
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

// MemorySecrets keeps secrets in memory only (tests, and the fallback when
// no OS store is available: passwords are then asked again after restart).
type MemorySecrets struct {
	mu sync.Mutex
	m  map[string]string
}

// NewMemorySecrets returns an empty in-memory store.
func NewMemorySecrets() *MemorySecrets { return &MemorySecrets{m: map[string]string{}} }

// Get returns the secret stored under key, and whether there is one.
func (s *MemorySecrets) Get(key string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[key]
	return v, ok, nil
}

// Set stores value under key.
func (s *MemorySecrets) Set(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = value
	return nil
}

// Delete removes the secret stored under key.
func (s *MemorySecrets) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, key)
	return nil
}

var (
	_ app.Store   = FileStore{}
	_ app.Secrets = (*MemorySecrets)(nil)
)
