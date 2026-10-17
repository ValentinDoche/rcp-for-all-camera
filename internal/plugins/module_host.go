// Package plugins loads the camera modules: one WebAssembly file per
// camera model in the modules folder, run by wazero in a sandbox. Each
// loaded file becomes a core.Module; each camera is its own instance of the
// module, which reaches its camera only through the services this package
// lends it (HTTP and WebSocket to the configured address, timers, events,
// log). docs/modules-camera.html describes the contract and the ABI.
package plugins

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/sdk/rcpmod"
)

// Limits of the sandbox.
const (
	maxModuleFile  = 64 << 20 // bytes of a .wasm file
	maxMessage     = 1 << 20  // bytes of a request, response or HTTP body
	defaultMemory  = 1024     // 64 KiB pages: 64 MiB per instance
	defaultTimeout = 10 * time.Second
)

// Options are the host's dependencies and limits.
type Options struct {
	// Logger receives module logs and load problems; nil discards them.
	Logger *slog.Logger
	// CacheDir keeps compiled modules between runs ("" = no cache).
	CacheDir string
	// CallTimeout bounds one call into a module (default 10 s). A module
	// that runs longer is stopped and its camera goes to the error state.
	CallTimeout time.Duration
	// MemoryPages bounds the memory of one instance, in 64 KiB pages
	// (default 1024, 64 MiB).
	MemoryPages uint32
}

// Host runs modules. It is safe for concurrent use; Close it when the
// application exits.
type Host struct {
	rt      wazero.Runtime
	log     *slog.Logger
	timeout time.Duration

	mu        sync.Mutex
	instances map[api.Module]*instance
}

// Open starts the WebAssembly runtime.
func Open(ctx context.Context, o Options) (*Host, error) {
	log := o.Logger
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	if o.CallTimeout <= 0 {
		o.CallTimeout = defaultTimeout
	}
	if o.MemoryPages == 0 {
		o.MemoryPages = defaultMemory
	}
	cfg := wazero.NewRuntimeConfig().WithMemoryLimitPages(o.MemoryPages).WithCloseOnContextDone(true)
	if o.CacheDir != "" {
		cache, err := wazero.NewCompilationCacheWithDir(o.CacheDir)
		if err != nil {
			log.Warn("module compilation cache unavailable", "err", err)
		} else {
			cfg = cfg.WithCompilationCache(cache)
		}
	}
	h := &Host{rt: wazero.NewRuntimeWithConfig(ctx, cfg), log: log, timeout: o.CallTimeout, instances: map[api.Module]*instance{}}
	if _, err := wasi_snapshot_preview1.Instantiate(ctx, h.rt); err != nil {
		_ = h.rt.Close(ctx)
		return nil, fmt.Errorf("wasi: %w", err)
	}
	_, err := h.rt.NewHostModuleBuilder("rcp").
		NewFunctionBuilder().WithFunc(h.hostCall).Export("call").
		NewFunctionBuilder().WithFunc(h.hostTake).Export("take").
		NewFunctionBuilder().WithFunc(h.hostLog).Export("log").
		Instantiate(ctx)
	if err != nil {
		_ = h.rt.Close(ctx)
		return nil, fmt.Errorf("host functions: %w", err)
	}
	return h, nil
}

// Close stops every instance and the runtime.
func (h *Host) Close(ctx context.Context) error { return h.rt.Close(ctx) }

// Load compiles every <id>.wasm of dir, a few at a time, and returns them
// in name order. A file that cannot be used (unreadable, malformed,
// declaring another id...) is skipped and reported; the others still load.
func (h *Host) Load(ctx context.Context, dir string) ([]core.Module, []error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, []error{fmt.Errorf("modules folder %s not found", dir)}
	}
	if err != nil {
		return nil, []error{fmt.Errorf("read modules folder: %w", err)}
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.EqualFold(filepath.Ext(e.Name()), ".wasm") {
			names = append(names, e.Name())
		}
	}
	mods := make([]core.Module, len(names))
	errs := make([]error, len(names))
	var wg sync.WaitGroup
	sem := make(chan struct{}, max(runtime.NumCPU(), 1))
	for i, name := range names {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			mods[i], errs[i] = h.LoadFile(ctx, filepath.Join(dir, name))
		}()
	}
	wg.Wait()
	var loaded []core.Module
	var problems []error
	for i, name := range names {
		if errs[i] != nil {
			problems = append(problems, fmt.Errorf("%s: %w", name, errs[i]))
			h.log.Warn("camera module skipped", "file", name, "err", errs[i])
			continue
		}
		loaded = append(loaded, mods[i])
	}
	return loaded, problems
}

// LoadFile compiles one module and reads its declaration. The file must be
// named after the module id.
func (h *Host) LoadFile(ctx context.Context, path string) (core.Module, error) {
	bin, err := readLimited(path, maxModuleFile)
	if err != nil {
		return nil, err
	}
	cm, err := h.rt.CompileModule(ctx, bin)
	if err != nil {
		return nil, fmt.Errorf("compile: %w", err)
	}
	for _, name := range []string{"rcp_alloc", "rcp_call"} {
		if _, ok := cm.ExportedFunctions()[name]; !ok {
			return nil, fmt.Errorf("not a camera module: no %s export", name)
		}
	}
	in, err := h.instantiate(ctx, cm, nil)
	if err != nil {
		return nil, err
	}
	defer in.close(ctx)
	var info rcpmod.Info
	if err := in.callResult(ctx, rcpmod.Request{Op: rcpmod.OpInfo}, &info); err != nil {
		return nil, fmt.Errorf("info: %w", err)
	}
	m, err := infoToCore(info)
	if err != nil {
		return nil, err
	}
	if base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)); base != string(m.ID) {
		return nil, fmt.Errorf("file is named %q but declares module %q", base, m.ID)
	}
	return &module{host: h, compiled: cm, info: m}, nil
}

func readLimited(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(b)) > limit {
		return nil, fmt.Errorf("larger than %d bytes", limit)
	}
	return b, nil
}

// module is a loaded camera module.
type module struct {
	host     *Host
	compiled wazero.CompiledModule
	info     core.ModelInfo
}

func (m *module) Info() core.ModelInfo {
	i := m.info
	i.Transports = slices.Clone(i.Transports)
	return i
}

// New builds a camera in a fresh instance of the module. Like every
// Module.New it does no network I/O: the instance only runs the module's
// constructor.
func (m *module) New(cfg core.CameraConfig) (core.Camera, error) {
	return newCamera(m, cfg)
}

// instance is one running copy of a module. Calls into it are serialized
// by mu; the host functions run inside such a call, on the same goroutine,
// and use svc without locking again.
type instance struct {
	host  *Host
	mod   api.Module
	alloc api.Function
	call  api.Function
	mu    sync.Mutex
	svc   *services // nil for the instance that only reads Info

	takes    map[uint32][]byte
	nextTake uint32
}

func (h *Host) instantiate(ctx context.Context, cm wazero.CompiledModule, svc *services) (*instance, error) {
	cfg := wazero.NewModuleConfig().WithName("").WithStartFunctions("_initialize").
		WithRandSource(rand.Reader).WithSysWalltime().WithSysNanotime()
	mod, err := h.rt.InstantiateModule(ctx, cm, cfg)
	if err != nil {
		return nil, fmt.Errorf("start module: %w", err)
	}
	in := &instance{host: h, mod: mod, alloc: mod.ExportedFunction("rcp_alloc"), call: mod.ExportedFunction("rcp_call"),
		svc: svc, takes: map[uint32][]byte{}}
	h.mu.Lock()
	h.instances[mod] = in
	h.mu.Unlock()
	return in, nil
}

func (in *instance) close(ctx context.Context) {
	in.host.mu.Lock()
	delete(in.host.instances, in.mod)
	in.host.mu.Unlock()
	_ = in.mod.Close(ctx)
}

// wireResponse is rcpmod.Response with the result left encoded.
type wireResponse struct {
	Result json.RawMessage   `json:"result"`
	Error  *rcpmod.WireError `json:"error"`
}

// invoke runs one request in the module; in.mu must be held. An error is
// a failure of the module itself (trap, timeout, garbage): the instance
// must not be used again. A module error is in the response.
func (in *instance) invoke(ctx context.Context, req rcpmod.Request) (wireResponse, error) {
	b, err := json.Marshal(req)
	if err != nil {
		return wireResponse{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, in.host.timeout)
	defer cancel()
	r, err := in.alloc.Call(ctx, uint64(len(b)))
	if err != nil {
		return wireResponse{}, fmt.Errorf("module stopped: %w", err)
	}
	if !in.mod.Memory().Write(uint32(r[0]), b) {
		return wireResponse{}, errors.New("module memory too small")
	}
	r, err = in.call.Call(ctx, r[0], uint64(len(b)))
	if err != nil {
		return wireResponse{}, fmt.Errorf("module stopped: %w", err)
	}
	ptr, n := uint32(r[0]>>32), uint32(r[0])
	if n > maxMessage {
		return wireResponse{}, errors.New("module answer too large")
	}
	out, ok := in.mod.Memory().Read(ptr, n)
	if !ok {
		return wireResponse{}, errors.New("module answer outside its memory")
	}
	var resp wireResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		return wireResponse{}, fmt.Errorf("module answer malformed: %w", err)
	}
	return resp, nil
}

// callResult runs a request that returns a result and decodes it into v;
// in.mu must be held (or the instance private to the caller).
func (in *instance) callResult(ctx context.Context, req rcpmod.Request, v any) error {
	resp, err := in.invoke(ctx, req)
	if err != nil {
		return err
	}
	if resp.Error != nil {
		return wireError(resp.Error)
	}
	if err := json.Unmarshal(resp.Result, v); err != nil {
		return fmt.Errorf("module result malformed: %w", err)
	}
	return nil
}

func (h *Host) lookup(m api.Module) *instance {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.instances[m]
}

// hostCall is the rcp.call import: it runs one HostRequest and keeps the
// answer for rcp.take. It returns id<<32 | length.
func (h *Host) hostCall(ctx context.Context, m api.Module, ptr, n uint32) uint64 {
	in := h.lookup(m)
	var resp rcpmod.HostResponse
	switch b, ok := m.Memory().Read(ptr, n); {
	case in == nil:
		resp.Error = "unknown instance"
	case !ok || n > maxMessage:
		resp.Error = "request outside memory or too large"
	default:
		var req rcpmod.HostRequest
		if err := json.Unmarshal(b, &req); err != nil {
			resp.Error = "malformed request"
		} else if in.svc == nil {
			resp.Error = "no camera in this instance"
		} else {
			resp = in.svc.handle(ctx, req)
		}
	}
	out, _ := json.Marshal(resp)
	if in == nil {
		return 0
	}
	in.nextTake++
	in.takes[in.nextTake] = out
	return uint64(in.nextTake)<<32 | uint64(len(out))
}

// hostTake is the rcp.take import: it copies a kept answer into the
// module's memory.
func (h *Host) hostTake(_ context.Context, m api.Module, id, ptr uint32) {
	in := h.lookup(m)
	if in == nil {
		return
	}
	b := in.takes[id]
	delete(in.takes, id)
	m.Memory().Write(ptr, b)
}

// maxLogLen bounds one module log line.
const maxLogLen = 1024

// hostLog is the rcp.log import.
func (h *Host) hostLog(ctx context.Context, m api.Module, level int32, ptr, n uint32) {
	b, ok := m.Memory().Read(ptr, min(n, maxLogLen))
	if !ok {
		return
	}
	msg := strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, string(b))
	lv := slog.Level(level)
	id := ""
	if in := h.lookup(m); in != nil && in.svc != nil {
		id = string(in.svc.cfg.Model)
	}
	h.log.Log(ctx, lv, msg, "module", id)
}
