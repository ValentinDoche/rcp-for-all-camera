//go:build windows

package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestOSSecretsRoundTripAcrossInstances(t *testing.T) {
	dir := t.TempDir()
	s, err := NewOSSecrets(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, err := s.Get("camera/1"); ok || err != nil {
		t.Fatalf("Get on empty store = %v, %v", ok, err)
	}
	if err := s.Set("camera/1", "hunter2"); err != nil {
		t.Fatal(err)
	}
	if err := s.Set("camera/2", "pässwörd"); err != nil {
		t.Fatal(err)
	}

	s2, err := NewOSSecrets(dir)
	if err != nil {
		t.Fatal(err)
	}
	if v, ok, err := s2.Get("camera/1"); err != nil || !ok || v != "hunter2" {
		t.Fatalf("Get after reopen = %q, %v, %v", v, ok, err)
	}
	if err := s2.Delete("camera/1"); err != nil {
		t.Fatal(err)
	}
	if err := s2.Delete("camera/1"); err != nil {
		t.Fatalf("second Delete = %v, want nil (idempotent)", err)
	}
	if _, ok, _ := s.Get("camera/1"); ok {
		t.Fatal("deleted secret still readable")
	}
	if v, _, _ := s.Get("camera/2"); v != "pässwörd" {
		t.Fatalf("other secret = %q", v)
	}
}

func TestOSSecretsFileHasNoPlaintext(t *testing.T) {
	dir := t.TempDir()
	s, err := NewOSSecrets(dir)
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Set("camera/1", "hunter2-plaintext-marker")
	b, err := os.ReadFile(filepath.Join(dir, SecretsFile))
	if err != nil {
		t.Fatal(err)
	}
	for _, needle := range []string{"hunter2-plaintext-marker", "camera/1"} {
		if bytes.Contains(b, []byte(needle)) {
			t.Fatalf("secrets file contains %q in clear", needle)
		}
	}
	matches, _ := filepath.Glob(filepath.Join(dir, ".secrets-*"))
	if len(matches) != 0 {
		t.Fatalf("temp files left behind: %v", matches)
	}
}

func TestOSSecretsRejectsTamperedOrForeignFile(t *testing.T) {
	cases := map[string]func(good []byte) []byte{
		"garbage":   func([]byte) []byte { return []byte("not a dpapi blob") },
		"bit flip":  func(g []byte) []byte { g = bytes.Clone(g); g[len(g)-5] ^= 0xFF; return g },
		"truncated": func(g []byte) []byte { return g[:len(g)/2] },
		"empty":     func([]byte) []byte { return nil },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			s, _ := NewOSSecrets(dir)
			_ = s.Set("camera/1", "pw")
			p := filepath.Join(dir, SecretsFile)
			good, _ := os.ReadFile(p)
			_ = os.WriteFile(p, mutate(good), 0o600)

			if _, err := NewOSSecrets(dir); !errors.Is(err, ErrSecretsUnreadable) {
				t.Fatalf("NewOSSecrets(%s) = %v, want ErrSecretsUnreadable", name, err)
			}
			if _, _, err := s.Get("camera/1"); !errors.Is(err, ErrSecretsUnreadable) {
				t.Fatalf("Get on %s file = %v, want ErrSecretsUnreadable", name, err)
			}
			if err := s.Set("camera/2", "x"); !errors.Is(err, ErrSecretsUnreadable) {
				t.Fatalf("Set on %s file = %v, want ErrSecretsUnreadable (never overwrite silently)", name, err)
			}
		})
	}
}

func TestOSSecretsRejectsOversizedFile(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, SecretsFile), make([]byte, MaxSecretsFileSize+1), 0o600)
	if _, err := NewOSSecrets(dir); !errors.Is(err, ErrSecretsUnreadable) {
		t.Fatalf("NewOSSecrets(oversized) = %v, want ErrSecretsUnreadable", err)
	}
}

func TestOSSecretsBoundsKeysAndValues(t *testing.T) {
	s, err := NewOSSecrets(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Set("", "x"); err == nil {
		t.Error("Set with empty key accepted")
	}
	if err := s.Set("k", string(make([]byte, maxSecretLen+1))); err == nil {
		t.Error("Set with oversized value accepted")
	}
}

func TestOSSecretsEntropyIsRequired(t *testing.T) {
	// A blob protected without the app entropy must not decrypt.
	blob, err := dpapiProtect([]byte(`{"version":1,"secrets":{}}`), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := dpapiUnprotect(blob, secretsEntropy); err == nil {
		t.Fatal("blob without entropy decrypted with the app entropy")
	}
}

func TestOSSecretsConcurrentUse(t *testing.T) {
	s, err := NewOSSecrets(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			k := fmt.Sprintf("camera/%d", i)
			if err := s.Set(k, k); err != nil {
				t.Error(err)
			}
			if v, ok, err := s.Get(k); err != nil || !ok || v != k {
				t.Errorf("Get(%s) = %q, %v, %v", k, v, ok, err)
			}
		}(i)
	}
	wg.Wait()
}
