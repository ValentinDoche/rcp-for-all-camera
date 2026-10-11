//go:build windows

// panelcheck drives the ATEM Micro Camera Panel through the app's panel
// controller (internal/surfaces/panel) with an in-memory fake rig, to check
// rendering and input mapping on hardware without cameras:
//
//	go run ./cmd/panelcheck -t 20s     3 cameras (slots 1, 2, 4), slot 2 active
//	go run ./cmd/panelcheck -layers    LED layer experiment (brightness?)
//
// Panel input is printed as the Service calls it would make. The panel is
// blanked (all LED layers cleared) on exit.
package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/valentindoche/rcp-for-all-camera/internal/app"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/micropanel"
	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/panel"
)

func main() {
	dur := flag.Duration("t", 20*time.Second, "run time (0 = until Ctrl+C)")
	layers := flag.Bool("layers", false, "LED layer experiment instead of the controller check")
	flag.Parse()

	if panel.OtherOwnerRunning() {
		log.Fatal("ATEM Software Control is running: close it first")
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if *dur > 0 {
		ctx, cancel = context.WithTimeout(ctx, *dur)
		defer cancel()
	}
	if *layers {
		layerExperiment(ctx)
		return
	}

	api := newFakeAPI()
	ctl := panel.New(panel.Options{Logger: slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug}))})
	api.emit = ctl.Emit
	ctl.Emit(app.EventCamera, app.CameraEvent{Slot: 4, Event: core.Event{Control: core.Tally, Value: core.Bool(true)}})
	ctl.Run(ctx, api, func(s app.SurfaceStatus) { log.Printf("panel status: %+v", s) })
	log.Print("done, panel blanked")
}

// fakeAPI is an in-memory rig: writes succeed and update the values.
type fakeAPI struct {
	mu     sync.Mutex
	rig    app.Rig
	specs  []core.ControlSpec
	values map[int]map[core.ControlID]core.Value
	emit   func(string, any)
}

func newFakeAPI() *fakeAPI {
	a := &fakeAPI{rig: app.Rig{Active: 2}, specs: fx6Specs(), values: map[int]map[core.ControlID]core.Value{}}
	for i := 1; i <= app.SlotCount; i++ {
		a.rig.Slots = append(a.rig.Slots, app.Slot{Slot: i})
	}
	for _, s := range []int{1, 2, 4} {
		st := core.StateConnected
		if s == 4 {
			st = core.StateError
		}
		a.rig.Slots[s-1].Camera = &app.CameraView{Model: "sony.fx6", Brand: "Sony", ModelName: "FX6",
			Name: fmt.Sprintf("Cam %d", s), Status: core.Status{State: st}}
		a.values[s] = map[core.ControlID]core.Value{
			core.Iris: core.Number(4), core.Gain: core.Number(6), core.ShutterSpeed: core.Choice("1/50"),
			core.ND: core.Choice("1/8"), core.MasterBlack: core.Number(3), core.ColorBars: core.Bool(false),
			core.GainR: core.Number(1), core.GainG: core.Number(1), core.GainB: core.Number(1),
			core.LiftR: core.Number(0), core.LiftG: core.Number(0), core.LiftB: core.Number(0),
		}
	}
	a.values[1][core.Iris] = core.Number(11) // to try the pickup on a camera switch
	return a
}

func (a *fakeAPI) Rig() app.Rig {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.rig
}

func (a *fakeAPI) Controls(slot int) ([]app.ControlView, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.rig.Slots[slot-1].Camera == nil {
		return nil, &app.Error{Code: app.CodeEmptySlot, Message: "empty"}
	}
	var out []app.ControlView
	for _, s := range a.specs {
		cv := app.ControlView{Spec: s}
		if v, ok := a.values[slot][s.ID]; ok {
			cv.Value = &v
		}
		out = append(out, cv)
	}
	return out, nil
}

func (a *fakeAPI) SetControl(slot int, id string, v core.Value) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	fmt.Printf("SetControl(%d, %s, %+v)\n", slot, id, v)
	if v.Kind != core.KindAction && a.values[slot] != nil {
		a.values[slot][core.ControlID(id)] = v
	}
	return nil
}

func (a *fakeAPI) SelectSlot(slot int) error {
	a.mu.Lock()
	fmt.Printf("SelectSlot(%d)\n", slot)
	a.rig.Active = slot
	rig := a.rig
	a.mu.Unlock()
	a.emit(app.EventRigChanged, rig)
	return nil
}

func fx6Specs() []core.ControlSpec {
	r := func(id core.ControlID, lo, hi, st float64, unit string) core.ControlSpec {
		s := core.Std(id)
		s.Kind, s.Min, s.Max, s.Step, s.Unit = core.KindRange, lo, hi, st, unit
		return s
	}
	e := func(id core.ControlID, opts ...string) core.ControlSpec {
		s := core.Std(id)
		s.Kind = core.KindEnum
		for _, o := range opts {
			s.Options = append(s.Options, core.Option{Value: o, Label: o})
		}
		return s
	}
	k := func(id core.ControlID, kind core.ControlKind) core.ControlSpec {
		s := core.Std(id)
		s.Kind = kind
		return s
	}
	return []core.ControlSpec{
		r(core.Iris, 1.4, 22, 0.1, "f"), k(core.AutoIris, core.KindAction),
		r(core.Gain, -3, 33, 1, "dB"),
		e(core.ShutterSpeed, "1/24", "1/25", "1/30", "1/50", "1/60", "1/100", "1/120", "1/250", "1/500", "1/1000", "1/2000"),
		e(core.ND, "clear", "1/4", "1/8", "1/16", "1/32", "1/64", "1/128"),
		r(core.MasterBlack, -15, 15, 1, ""),
		k(core.AutoWhiteBalance, core.KindAction), k(core.ColorBars, core.KindToggle),
		r(core.GainR, 0, 16, 0.01, ""), r(core.GainG, 0, 16, 0.01, ""), r(core.GainB, 0, 16, 0.01, ""),
		r(core.LiftR, -2, 2, 0.01, ""), r(core.LiftG, -2, 2, 0.01, ""), r(core.LiftB, -2, 2, 0.01, ""),
		k(core.Tally, core.KindToggle),
	}
}

// layerExperiment lights camera button n (1..6) white on LED layer n-1 only,
// so one photo shows whether layers change brightness or blink. Displays
// read "LEd" on SHUTTER. Read-only features 0x04, 0x06, 0x15 are dumped.
func layerExperiment(ctx context.Context) {
	p, err := micropanel.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer p.Close()
	for _, f := range []struct {
		id   byte
		size int
	}{{0x04, 8}, {0x06, 3}, {0x15, 33}} {
		if b, err := p.GetFeature(f.id, f.size); err == nil {
			log.Printf("feature 0x%02X: %s %q", f.id, hex.EncodeToString(b), b[1:])
		} else {
			log.Printf("feature 0x%02X: %v", f.id, err)
		}
	}
	for layer := 0; layer < micropanel.LEDLayers; layer++ {
		var l micropanel.LEDBits
		l.SetCamera(layer+1, micropanel.White)
		if err := p.WriteReport(l.Report(byte(layer))); err != nil {
			log.Fatal(err)
		}
	}
	var seg [14]byte
	micropanel.EncodeText(&seg, micropanel.DispShutter, "LEd")
	p.SetDigitsRaw(seg)
	log.Print("layers: camera button n is white on layer n-1; waiting...")
	<-ctx.Done()
	p.SetDigitsRaw([14]byte{})
	if err := p.ClearLEDs(-1); err != nil {
		log.Print(err)
	}
	log.Print("done, all layers cleared")
}
