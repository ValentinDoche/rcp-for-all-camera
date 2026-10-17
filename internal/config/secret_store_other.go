//go:build !windows

package config

import "github.com/valentindoche/rcp-for-all-camera/internal/app"

// NewOSSecrets returns ErrNoOSSecrets outside Windows; callers fall back to
// MemorySecrets (passwords are then asked again after a restart).
func NewOSSecrets(dir string) (app.Secrets, error) { return nil, ErrNoOSSecrets }
