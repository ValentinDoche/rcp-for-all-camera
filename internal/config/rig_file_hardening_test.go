package config

import (
	"os"
	"path/filepath"
	"testing"
)

// A rig that cannot be loaded must be kept aside: the app starts with an
// empty rig and its first Save would otherwise overwrite the user's file.
func TestLoadMovesUnreadableFileAside(t *testing.T) {
	for name, content := range map[string]string{
		"malformed":   "{not json",
		"new version": `{"version":99}`,
	} {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "rig.json")
			_ = os.WriteFile(p, []byte(content), 0o600)
			if _, err := (FileStore{Path: p}).Load(); err == nil {
				t.Fatal("Load = nil, want error")
			}
			b, err := os.ReadFile(p + ".bad")
			if err != nil || string(b) != content {
				t.Fatalf("backup = %q, %v; want the original content", b, err)
			}
			if _, err := os.Stat(p); !os.IsNotExist(err) {
				t.Fatalf("unreadable rig still at its path (err %v)", err)
			}
		})
	}
}
