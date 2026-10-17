// Package pluginstest gives tests the real camera modules: it builds every
// folder of modules/ to WebAssembly once per test process and loads them
// in a plugin host, exactly as the application does.
package pluginstest

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/modbuild"
	"github.com/valentindoche/rcp-for-all-camera/internal/plugins"
)

var (
	once sync.Once
	mods []core.Module
	err  error
)

// Modules returns every module of the source tree, loaded. The first call
// builds them (a few seconds, then the Go build cache keeps them); the host
// stays open for the whole test process.
func Modules(t testing.TB) []core.Module {
	t.Helper()
	once.Do(load)
	if err != nil {
		t.Fatalf("load the camera modules: %v", err)
	}
	return mods
}

func load() {
	_, file, _, _ := runtime.Caller(0)
	src := filepath.Join(filepath.Dir(file), "..", "..", "..", "modules")
	dir, e := os.MkdirTemp("", "rcp-modules-")
	if e != nil {
		err = e
		return
	}
	ctx := context.Background()
	if err = modbuild.Build(ctx, src, dir); err != nil {
		return
	}
	cache := filepath.Join(os.TempDir(), "rcp-module-cache")
	h, e := plugins.Open(ctx, plugins.Options{CacheDir: cache})
	if e != nil {
		err = e
		return
	}
	var problems []error
	mods, problems = h.Load(ctx, dir)
	if len(problems) > 0 {
		err = problems[0]
	}
}
