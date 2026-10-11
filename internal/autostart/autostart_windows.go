// Package autostart registers the app to start with the Windows session,
// through the per-user Run key (no admin rights needed).
package autostart

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

// RunKey is the per-user key Windows reads at logon.
const RunKey = `Software\Microsoft\Windows\CurrentVersion\Run`

// Entry is one Run value: Name -> "Exe".
type Entry struct {
	KeyPath string
	Name    string
	Exe     string
}

// ForThisExe returns the entry for the running executable.
func ForThisExe() (*Entry, error) {
	exe, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("autostart: %w", err)
	}
	if p, err := filepath.EvalSymlinks(exe); err == nil {
		exe = p
	}
	return &Entry{KeyPath: RunKey, Name: "RCPForAllCamera", Exe: exe}, nil
}

func (e *Entry) value() string { return `"` + e.Exe + `"` }

// Enabled reports whether the Run value exists and points to this Exe.
func (e *Entry) Enabled() (bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, e.KeyPath, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("autostart: open key: %w", err)
	}
	defer k.Close()
	v, _, err := k.GetStringValue(e.Name)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("autostart: read value: %w", err)
	}
	return v == e.value(), nil
}

// SetEnabled writes or removes the Run value. Removing a missing value succeeds.
func (e *Entry) SetEnabled(on bool) error {
	if !on {
		k, err := registry.OpenKey(registry.CURRENT_USER, e.KeyPath, registry.SET_VALUE)
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("autostart: open key: %w", err)
		}
		defer k.Close()
		if err := k.DeleteValue(e.Name); err != nil && !errors.Is(err, registry.ErrNotExist) {
			return fmt.Errorf("autostart: delete value: %w", err)
		}
		return nil
	}
	k, _, err := registry.CreateKey(registry.CURRENT_USER, e.KeyPath, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("autostart: create key: %w", err)
	}
	defer k.Close()
	if err := k.SetStringValue(e.Name, e.value()); err != nil {
		return fmt.Errorf("autostart: write value: %w", err)
	}
	return nil
}
