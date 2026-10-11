//go:build windows

package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"unicode/utf8"
)

// osSecrets keeps the secrets in <dir>\secrets.dat, encrypted with DPAPI for
// the current Windows user. Nothing is cached: every call decrypts the file,
// so the decrypted buffer only lives for the duration of the call (Go
// strings returned by Get cannot be wiped).
type osSecrets struct {
	mu   sync.Mutex
	path string
}

// NewOSSecrets opens the DPAPI-protected store in dir (created if needed).
// It fails with ErrSecretsUnreadable when an existing file cannot be
// decrypted (corrupt, or from another user or machine) and with
// ErrNoOSSecrets when DPAPI is missing. The caller decides what to do:
// typically log, fall back to MemorySecrets, and tell the user.
func NewOSSecrets(dir string) (Secrets, error) {
	if dir == "" {
		return nil, errors.New("secrets: empty directory")
	}
	if err := dpapiAvailable(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNoOSSecrets, err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create secrets dir: %w", err)
	}
	s := &osSecrets{path: filepath.Join(dir, SecretsFile)}
	if _, err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *osSecrets) Get(key string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, err := s.load()
	if err != nil {
		return "", false, err
	}
	v, ok := m[key]
	return v, ok, nil
}

func (s *osSecrets) Set(key, value string) error {
	if err := checkSecret(key, value); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	m, err := s.load()
	if err != nil {
		return err
	}
	if _, exists := m[key]; !exists && len(m) >= maxSecrets {
		return fmt.Errorf("secrets: more than %d entries", maxSecrets)
	}
	m[key] = value
	return s.save(m)
}

func (s *osSecrets) Delete(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	m, err := s.load()
	if err != nil {
		return err
	}
	if _, ok := m[key]; !ok {
		return nil
	}
	delete(m, key)
	return s.save(m)
}

func checkSecret(key, value string) error {
	if key == "" || len(key) > maxSecretKeyLen || !utf8.ValidString(key) {
		return fmt.Errorf("secrets: key must be 1 to %d bytes of UTF-8", maxSecretKeyLen)
	}
	if len(value) > maxSecretLen || !utf8.ValidString(value) {
		return fmt.Errorf("secrets: value must be at most %d bytes of UTF-8", maxSecretLen)
	}
	return nil
}

// load decrypts the file; a missing file is an empty store. Errors never
// include decrypted content.
func (s *osSecrets) load() (map[string]string, error) {
	f, err := os.Open(s.path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]string{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open secrets: %w", err)
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, MaxSecretsFileSize+1))
	if err != nil {
		return nil, fmt.Errorf("read secrets: %w", err)
	}
	if len(b) > MaxSecretsFileSize {
		return nil, fmt.Errorf("larger than %d bytes: %w", MaxSecretsFileSize, ErrSecretsUnreadable)
	}
	if !bytes.HasPrefix(b, secretsMagic) {
		return nil, fmt.Errorf("bad header: %w", ErrSecretsUnreadable)
	}
	plain, err := dpapiUnprotect(b[len(secretsMagic):], secretsEntropy)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSecretsUnreadable, err)
	}
	defer clear(plain)
	var doc secretsDoc
	// The JSON error is dropped on purpose: it may quote decrypted bytes.
	if json.Unmarshal(plain, &doc) != nil || doc.Version != secretsVersion || len(doc.Secrets) > maxSecrets {
		return nil, fmt.Errorf("bad content: %w", ErrSecretsUnreadable)
	}
	if doc.Secrets == nil {
		doc.Secrets = map[string]string{}
	}
	return doc.Secrets, nil
}

func (s *osSecrets) save(m map[string]string) error {
	plain, err := json.Marshal(secretsDoc{Version: secretsVersion, Secrets: m})
	if err != nil {
		return fmt.Errorf("encode secrets: %w", err)
	}
	defer clear(plain)
	blob, err := dpapiProtect(plain, secretsEntropy)
	if err != nil {
		return fmt.Errorf("encrypt secrets: %w", err)
	}
	return writeFileAtomic(s.path, ".secrets-*.tmp", append(bytes.Clone(secretsMagic), blob...))
}
