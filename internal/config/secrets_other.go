//go:build !windows

package config

// NewOSSecrets returns ErrNoOSSecrets outside Windows; callers fall back to
// MemorySecrets (passwords are then asked again after a restart).
func NewOSSecrets(dir string) (Secrets, error) { return nil, ErrNoOSSecrets }
