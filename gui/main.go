// Command gui is the RCP For All Camera desktop app (Wails v2).
//
//	wails build            (from this directory)
//	go build -tags desktop,production -ldflags "-H windowsgui" -o rcp.exe ./gui
package main

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/valentindoche/rcp-for-all-camera/internal/app"
	"github.com/valentindoche/rcp-for-all-camera/internal/cameras"
	"github.com/valentindoche/rcp-for-all-camera/internal/config"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

//go:embed all:frontend/dist
var assets embed.FS

const panelPollInterval = time.Second

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "rcp:", err)
		os.Exit(1)
	}
}

func run() error {
	rigPath, err := config.DefaultPath()
	if err != nil {
		return err
	}
	logger, closeLog := openLog(filepath.Join(filepath.Dir(rigPath), "rcp.log"))
	defer closeLog()

	reg := core.NewRegistry()
	if err := cameras.Register(reg); err != nil {
		return err
	}
	svc, err := app.NewService(app.Options{
		Registry: reg,
		Store:    config.FileStore{Path: rigPath},
		Secrets:  newSecrets(filepath.Dir(rigPath), logger),
		Logger:   logger,
	})
	if err != nil {
		return err
	}
	host := app.NewHost(svc, panelProbe, panelPollInterval)

	return wails.Run(&options.App{
		Title:            "RCP For All Camera",
		Width:            1280,
		Height:           800,
		MinWidth:         1024,
		MinHeight:        640,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 24, G: 24, B: 27, A: 255},
		OnStartup: func(ctx context.Context) {
			host.Start(ctx, wailsEmitter{ctx})
		},
		OnShutdown: func(context.Context) { host.Stop() },
		Bind:       []interface{}{host.Service()},
	})
}

type wailsEmitter struct{ ctx context.Context }

func (e wailsEmitter) Emit(name string, payload any) { runtime.EventsEmit(e.ctx, name, payload) }

// openLog writes logs to a file next to the config (the GUI has no console).
// It falls back to stderr if the file cannot be opened.
func openLog(path string) (*slog.Logger, func()) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err == nil {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
		if err == nil {
			return slog.New(slog.NewTextHandler(f, nil)), func() { f.Close() }
		}
	}
	return slog.New(slog.NewTextHandler(os.Stderr, nil)), func() {}
}
