// Command rcp is the RCP For All Camera desktop application (Fyne GUI).
package main

// The exe icon: rsrc_windows_amd64.syso, built from the app logo.
//go:generate go run ./winres

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	appsvc "github.com/valentindoche/rcp-for-all-camera/internal/app"
	"github.com/valentindoche/rcp-for-all-camera/internal/config"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/gui"
	"github.com/valentindoche/rcp-for-all-camera/internal/plugins"
	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/panel"
	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/panelsim"
	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/panelsource"
	"github.com/valentindoche/rcp-for-all-camera/internal/sysinfo"
)

// version is set by release builds (-ldflags "-X main.version=v1.2.3").
var version = "dev"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "rcp:", err)
		os.Exit(1)
	}
}

func run() error {
	sim := flag.Bool("sim", false, "start on the simulated Micro Camera Panel (this run only)")
	modulesDir := flag.String("modules", "", "folder of the camera modules (default: modules next to rcp.exe)")
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

	reg, closeModules, err := loadModules(*modulesDir, log)
	if err != nil {
		return err
	}
	defer closeModules()
	secrets, err := config.NewOSSecrets(dir)
	if err != nil {
		// Passwords then only live until the app closes.
		log.Warn("secret store unavailable, passwords kept in memory", "err", err)
		secrets = config.NewMemorySecrets()
	}
	// The panel comes from USB (first or chosen panel) or the simulator,
	// selected in the GUI and saved in rig.json.
	simPanel := panelsim.New()
	sources := panelsource.New(simPanel)
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
		Panels:    sources,
	})
	if err != nil {
		return err
	}
	if *sim { // for this run only, not saved
		_ = sources.Select(appsvc.PanelSourceSim)
	}
	// The panel is a control surface: rendered from and driving the Service.
	ctl := panel.New(panel.Options{Logger: log, Open: sources.Open, Busy: sources.Busy})
	host := appsvc.NewHost(svc, sources.Present, time.Second)
	host.SetSurface(ctl)

	fa := app.NewWithID(gui.AppID)
	win := gui.New(fa, svc, gui.Options{Version: version, Stats: hostStats{sysinfo.New()}})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var simWin fyne.Window // created once the Fyne loop runs
	showSim := func(id string) {
		if simWin == nil {
			return
		}
		if id == appsvc.PanelSourceSim {
			simWin.Show()
		} else {
			simWin.Hide()
		}
	}
	sources.OnSelect(func(id string) {
		log.Info("panel source selected", "source", id)
		ctl.Reconnect()
		fyne.Do(func() { showSim(id) })
	})
	// events may only be delivered once the Fyne loop runs
	fa.Lifecycle().SetOnStarted(func() {
		host.Start(ctx, win)
		// Created after the main window, which stays the master window.
		simWin = panelsim.NewWindow(fa, simPanel)
		showSim(sources.Selected())
	})
	defer host.Stop()
	win.Run(ctx) // blocks until the window closes
	return nil
}

// loadModules loads the camera modules of dir (default: the modules folder
// next to the executable) into a new registry. A module that cannot be
// loaded is logged and skipped.
func loadModules(dir string, log *slog.Logger) (*core.Registry, func(), error) {
	if dir == "" {
		exe, err := os.Executable()
		if err != nil {
			return nil, nil, fmt.Errorf("find the modules folder: %w", err)
		}
		dir = filepath.Join(filepath.Dir(exe), "modules")
	}
	cache := ""
	if d, err := os.UserCacheDir(); err == nil {
		cache = filepath.Join(d, "RCPForAllCamera", "modules")
	}
	ctx := context.Background()
	host, err := plugins.Open(ctx, plugins.Options{Logger: log, CacheDir: cache})
	if err != nil {
		return nil, nil, err
	}
	mods, problems := host.Load(ctx, dir)
	log.Info("camera modules loaded", "folder", dir, "modules", len(mods), "skipped", len(problems))
	reg := core.NewRegistry()
	if err := reg.Register(mods...); err != nil {
		_ = host.Close(ctx)
		return nil, nil, err
	}
	return reg, func() { _ = host.Close(ctx) }, nil
}

// hostStats adapts sysinfo to the status bar's port.
type hostStats struct{ s *sysinfo.Sampler }

func (h hostStats) Sample() (gui.HostStats, error) {
	st, err := h.s.Sample()
	return gui.HostStats{CPU: st.CPU, MemUsed: st.MemUsed, MemTotal: st.MemTotal, DownBps: st.DownBps,
		UpBps: st.UpBps, IP: st.IP, HasRates: st.HasRates}, err
}
