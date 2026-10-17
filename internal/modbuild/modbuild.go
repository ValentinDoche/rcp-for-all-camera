// Package modbuild compiles the camera modules of the source tree: every
// folder of modules/ that holds a main.go (internal/ excepted) becomes
// <folder>.wasm. cmd/buildmodules and the tests that load real modules use
// it.
package modbuild

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sync"
)

// List returns the module folders of srcDir, sorted.
func List(srcDir string) ([]string, error) {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", srcDir, err)
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "internal" {
			continue
		}
		if _, err := os.Stat(filepath.Join(srcDir, e.Name(), "main.go")); err == nil {
			out = append(out, e.Name())
		}
	}
	slices.Sort(out)
	return out, nil
}

// Build compiles the named module folders of srcDir (all of them when
// names is empty) into outDir/<name>.wasm, a few at a time.
func Build(ctx context.Context, srcDir, outDir string, names ...string) error {
	all, err := List(srcDir)
	if err != nil {
		return err
	}
	if len(names) == 0 {
		names = all
	}
	for _, n := range names {
		if !slices.Contains(all, n) {
			return fmt.Errorf("no module folder %q in %s", n, srcDir)
		}
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	src, err := filepath.Abs(srcDir)
	if err != nil {
		return err
	}
	out, err := filepath.Abs(outDir)
	if err != nil {
		return err
	}
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		errs []error
		sem  = make(chan struct{}, 4)
	)
	for _, n := range names {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if err := buildOne(ctx, filepath.Join(src, n), filepath.Join(out, n+".wasm")); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("%s: %w", n, err))
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	return errors.Join(errs...)
}

func buildOne(ctx context.Context, dir, out string) error {
	cmd := exec.CommandContext(ctx, "go", "build", "-buildmode=c-shared", "-trimpath", "-ldflags=-s -w", "-o", out, ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm", "CGO_ENABLED=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w\n%s", err, stderr.String())
	}
	return nil
}
