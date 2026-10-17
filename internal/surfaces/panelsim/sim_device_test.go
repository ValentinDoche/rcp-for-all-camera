package panelsim_test

import (
	"context"
	"errors"
	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/panelsim"
	"slices"
	"testing"
	"time"

	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/micropanel"
	"github.com/valentindoche/rcp-for-all-camera/internal/surfaces/panel"
)

// session runs the sim and returns the states it delivers.
func session(t *testing.T, s *panelsim.Sim) (<-chan micropanel.State, <-chan error, context.CancelFunc) {
	t.Helper()
	dev, err := s.Open()
	if err != nil {
		t.Fatal(err)
	}
	states := make(chan micropanel.State, 16)
	errc := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	go func() { errc <- dev.Run(ctx, func(st micropanel.State) { states <- st }, nil) }()
	return states, errc, cancel
}

func next(t *testing.T, c <-chan micropanel.State) micropanel.State {
	t.Helper()
	select {
	case st := <-c:
		return st
	case <-time.After(2 * time.Second):
		t.Fatal("no state delivered")
	}
	return micropanel.State{}
}

func TestRunDeliversInitialStateThenChanges(t *testing.T) {
	s := panelsim.New()
	states, errc, cancel := session(t, s)
	first := next(t, states)
	if !first.AnalogRead || first.Analog[0] != 512 {
		t.Fatalf("first state = %+v, want knobs read at 512", first)
	}
	s.Press(0x40, true)
	if st := next(t, states); !slices.Equal(st.Keys, []micropanel.Key{0x40}) {
		t.Fatalf("keys = %v", st.Keys)
	}
	s.Press(0x40, false)
	if st := next(t, states); len(st.Keys) != 0 {
		t.Fatalf("keys after release = %v", st.Keys)
	}
	s.SetLever(5000)
	if st := next(t, states); st.Lever != micropanel.JoystickMax {
		t.Fatalf("lever = %d, want clamped to 4096", st.Lever)
	}
	cancel()
	if err := <-errc; !errors.Is(err, context.Canceled) {
		t.Fatalf("Run = %v", err)
	}
}

func TestKnobCounterClamps(t *testing.T) {
	s := panelsim.New()
	s.TurnKnob(3, 2000)
	s.TurnKnob(4, -2000)
	in := s.Input()
	if in.Analog[3] != 1024 || in.Analog[4] != 0 {
		t.Fatalf("analog = %v", in.Analog)
	}
}

func TestUnplugEndsSessionAndHidesDevice(t *testing.T) {
	s := panelsim.New()
	states, errc, _ := session(t, s)
	next(t, states)
	if err := s.SetDigitsRaw([14]byte{1}); err != nil {
		t.Fatal(err)
	}
	s.SetPlugged(false)
	if err := <-errc; !errors.Is(err, panelsim.ErrUnplugged) {
		t.Fatalf("Run = %v, want unplugged", err)
	}
	if s.Output().Digits != [14]byte{} {
		t.Fatal("unplugged panel still shows digits")
	}
	if _, err := s.Open(); !errors.Is(err, panel.ErrNoDevice) {
		t.Fatalf("Open = %v, want ErrNoDevice", err)
	}
	s.SetPlugged(true)
	if _, err := s.Open(); err != nil {
		t.Fatal(err)
	}
}

func TestLEDLayersAreORed(t *testing.T) {
	s := panelsim.New()
	var a, b micropanel.LEDBits
	a.Set(micropanel.LEDStoreRed, true)
	b.Set(micropanel.LEDAlarm, true)
	for _, w := range [][]byte{a.Report(0), b.Report(3)} {
		if err := s.WriteReport(w); err != nil {
			t.Fatal(err)
		}
	}
	o := s.Output()
	if !o.LED(micropanel.LEDStoreRed) || !o.LED(micropanel.LEDAlarm) {
		t.Fatal("layers not combined")
	}
	if err := s.ClearLEDs(0); err != nil {
		t.Fatal(err)
	}
	if o := s.Output(); !o.LED(micropanel.LEDStoreRed) || o.LED(micropanel.LEDAlarm) {
		t.Fatal("ClearLEDs(0) must keep layer 0 only")
	}
}

func TestFeatureReports(t *testing.T) {
	s := panelsim.New()
	var pos [9]uint16
	pos[0], pos[8] = 100, 2000
	if err := s.SetFeature(micropanel.AnalogReport(pos)); err != nil {
		t.Fatal(err)
	}
	b, err := s.GetFeature(0x0E, 19)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := micropanel.ParseAnalog(b)
	if got[0] != 100 || got[8] != 1024 {
		t.Fatalf("knobs = %v, want 100 and clamped 1024", got)
	}
	s.SetRing(-300)
	b, err = s.GetFeature(0x0F, 13)
	if err != nil || len(b) != 13 {
		t.Fatalf("0x0F = % x, %v", b, err)
	}
	if _, err := s.GetFeature(0x0E, 64); err == nil {
		t.Fatal("a padded feature read must fail (trap 1)")
	}
}

// The real controller drives the sim: camera keys select a slot.
func TestControllerDrivesSim(t *testing.T) {
	s := panelsim.New()
	c := panel.New(panel.Options{Open: s.Open, Busy: func() bool { return false },
		FrameEvery: 5 * time.Millisecond, Throttle: 5 * time.Millisecond, RetryMin: 5 * time.Millisecond})
	api := &fakeAPI{}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { c.Run(ctx, api, nil); close(done) }()
	defer func() { cancel(); <-done }()

	waitFor(t, func() bool { return s.Output().Matrix != [32]byte{} }) // slot 1 number drawn
	waitFor(t, s.InSession)
	s.Press(0x41, true)
	s.Press(0x41, false)
	waitFor(t, func() bool { return api.selected() == 2 })
}

func waitFor(t *testing.T, ok func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !ok() {
		if time.Now().After(deadline) {
			t.Fatal("condition not reached")
		}
		time.Sleep(5 * time.Millisecond)
	}
}
