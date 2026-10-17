// Command rcp is the RCP For All Camera desktop application (Fyne GUI).
package main

import (
	"context"
	"flag"
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
	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/panel"
	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/panelsim"
	"github.com/valentindoche/rcp-for-all-camera/internal/sysinfo"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "rcp:", err)
		os.Exit(1)
	}
}

func run() error {
	sim := flag.Bool("sim", false, "drive a simulated Micro Camera Panel window instead of the USB panel")
	flag.Parse()

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
	level := new(slog.LevelVar) // Info until the saved setting is applied
	log := slog.New(slog.NewTextHandler(logFile, &slog.HandlerOptions{Level: level}))

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
		LogLevel:  level,
		LogDir:    dir,
	})
	if err != nil {
		return err
	}
	// The USB panel is a control surface: rendered from and driving the
	// Service. With -sim the same controller drives a simulated panel.
	probe, surf := appsvc.PanelProbe(panelProbe), panel.Options{Logger: log}
	var simPanel *panelsim.Sim
	if *sim {
		simPanel = panelsim.New()
		probe = simPanel.Present
		surf.Open, surf.Busy = simPanel.Open, func() bool { return false }
		log.Info("panel simulator enabled")
	}
	host := appsvc.NewHost(svc, probe, time.Second)
	host.SetSurface(panel.New(surf))

	fa := app.NewWithID(gui.AppID)
	win := gui.New(fa, svc, gui.Options{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// events may only be delivered once the Fyne loop runs
	fa.Lifecycle().SetOnStarted(func() {
		host.Start(ctx, win)
		if simPanel != nil {
			// Shown after the main window, which stays the master window.
			panelsim.NewWindow(fa, simPanel).Show()
		}
	})
	defer host.Stop()
	win.Run(ctx) // blocks until the window closes
	return nil
}
