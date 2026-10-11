// Command rcp is the RCP For All Camera desktop application (Fyne GUI).
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2/app"

	appsvc "github.com/valentindoche/rcp-for-all-camera/internal/app"
	"github.com/valentindoche/rcp-for-all-camera/internal/cameras"
	"github.com/valentindoche/rcp-for-all-camera/internal/config"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui"
	"github.com/valentindoche/rcp-for-all-camera/internal/sysinfo"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "rcp:", err)
		os.Exit(1)
	}
}

func run() error {
	cfgPath, err := config.DefaultPath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(cfgPath)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	logFile, err := os.OpenFile(filepath.Join(dir, "rcp.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("open log: %w", err)
	}
	defer logFile.Close()
	log := slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{Level: slog.LevelInfo}))

	reg := core.NewRegistry()
	if err := cameras.Register(reg); err != nil {
		return err
	}
	secrets, err := config.NewOSSecrets(dir)
	if err != nil {
		// Passwords then only live until the app closes.
		log.Warn("secret store unavailable, passwords kept in memory", "err", err)
		secrets = config.NewMemorySecrets()
	}
	svc, err := appsvc.NewService(appsvc.Options{
		Registry: reg,
		Store:    config.FileStore{Path: cfgPath},
		Secrets:  secrets,
		Logger:   log,
		Interfaces: func() []appsvc.NetInterface {
			var out []appsvc.NetInterface
			for _, n := range sysinfo.LocalIPv4s() {
				out = append(out, appsvc.NetInterface{Name: n.Name, IP: n.IP})
			}
			return out
		},
		AutoStart: autoStarter(),
	})
	if err != nil {
		return err
	}
	host := appsvc.NewHost(svc, panelProbe, time.Second)

	fa := app.NewWithID(gui.AppID)
	win := gui.New(fa, svc, gui.Options{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// events may only be delivered once the Fyne loop runs
	fa.Lifecycle().SetOnStarted(func() { host.Start(ctx, win) })
	defer host.Stop()
	win.Run(ctx) // blocks until the window closes
	return nil
}
