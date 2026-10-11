package widgets

import (
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

func near(a, b, eps float64) bool { return math.Abs(a-b) <= eps }

func TestPointToUnitClampsToDisc(t *testing.T) {
	// centre (100,100), radius 50, screen y grows downward
	cases := []struct{ px, py, wx, wy float64 }{
		{100, 100, 0, 0},
		{150, 100, 1, 0},
		{100, 50, 0, 1},  // up on screen = +y
		{200, 100, 1, 0}, // clamped
		{200, 0, math.Sqrt2 / 2, math.Sqrt2 / 2},
		{75, 100, -0.5, 0},
	}
	for _, c := range cases {
		x, y := PointToUnit(c.px, c.py, 100, 100, 50)
		if !near(x, c.wx, 1e-9) || !near(y, c.wy, 1e-9) {
			t.Errorf("PointToUnit(%v,%v) = %v,%v want %v,%v", c.px, c.py, x, y, c.wx, c.wy)
		}
	}
}

func TestPuckRGBRoundTrip(t *testing.T) {
	const span = 0.5
	for _, p := range [][2]float64{{0, 0}, {0.5, 0}, {0, 0.7}, {-0.3, -0.4}, {0.6, -0.6}, {-1, 0}} {
		r, g, b := PuckToRGB(p[0], p[1], 0.1, span, -2, 2)
		x, y := RGBToPuck(r, g, b, 0.1, span)
		if !near(x, p[0], 1e-9) || !near(y, p[1], 1e-9) {
			t.Errorf("round trip %v -> (%v,%v,%v) -> %v,%v", p, r, g, b, x, y)
		}
	}
}

func TestPuckCentreIsMasterOnly(t *testing.T) {
	r, g, b := PuckToRGB(0, 0, 1, 1, 0, 16)
	if r != 1 || g != 1 || b != 1 {
		t.Errorf("centre = %v %v %v, want all equal to Y=1", r, g, b)
	}
}

func TestPuckDirectionsFavourChannel(t *testing.T) {
	r, g, b := PuckToRGB(1, 0, 0, 0.5, -2, 2) // hue 0 = red
	if !(r > 0 && g < 0 && b < 0) || !near(r, 0.5, 1e-9) {
		t.Errorf("red direction = %v %v %v", r, g, b)
	}
	r, g, b = PuckToRGB(-0.5, math.Sqrt(3)/2, 0, 1, -4, 4) // hue 120 = green
	if !(g > r && g > b) || !near(g, 1, 1e-9) {
		t.Errorf("green direction = %v %v %v", r, g, b)
	}
}

func TestPuckClampsToSpecRange(t *testing.T) {
	r, g, b := PuckToRGB(1, 0, 0, 1, 0, 0.2) // would be 1, -0.5, -0.5
	if r != 0.2 || g != 0 || b != 0 {
		t.Errorf("clamp = %v %v %v", r, g, b)
	}
}

func TestWheelSpan(t *testing.T) {
	cases := []struct{ min, max, want float64 }{{-2, 2, 0.5}, {-4, 4, 1}, {0, 16, 1}, {0, 2, 0.25}, {0, 0, 1}}
	for _, c := range cases {
		if got := WheelSpan(c.min, c.max); !near(got, c.want, 1e-9) {
			t.Errorf("WheelSpan(%v,%v) = %v want %v", c.min, c.max, got, c.want)
		}
	}
}

func TestNeutralValues(t *testing.T) {
	if Neutral(core.LiftY, -2, 2) != 0 || Neutral(core.GammaR, -4, 4) != 0 || Neutral(core.GainB, 0, 16) != 1 {
		t.Error("standard neutrals")
	}
	if Neutral(core.GainY, 2, 4) != 2 { // clamped into the range
		t.Error("neutral must be clamped into [min,max]")
	}
}

func TestQuantize(t *testing.T) {
	r := Range{Min: 0, Max: 1, Step: 0.01}
	for in, want := range map[float64]float64{0.123: 0.12, 0.126: 0.13, -1: 0, 2: 1} {
		if got := Quantize(in, r); !near(got, want, 1e-9) {
			t.Errorf("Quantize(%v) = %v want %v", in, got, want)
		}
	}
	r = Range{Min: 11.2, Max: 360, Step: 0}
	if got := Quantize(500, r); got != 360 {
		t.Errorf("no step clamps only, got %v", got)
	}
	r = Range{Min: -12, Max: 36, Step: 2}
	if got := Quantize(5, r); got != 6 && got != 4 {
		t.Errorf("step 2 got %v", got)
	}
	if got := Quantize(5.1, Range{Min: -12, Max: 36, Step: 2}); got != 6 {
		t.Errorf("5.1 -> %v want 6 (steps counted from min)", got)
	}
}

func TestPadMapping(t *testing.T) {
	iris := &Range{Min: 0, Max: 1, Step: 0.01}
	mb := &Range{Min: -150, Max: 150, Step: 1}
	i, m := PadToValues(1, 1, iris, mb)
	if i == nil || *i != 1 || m == nil || *m != 150 {
		t.Fatalf("top-right = %v %v", i, m)
	}
	i, m = PadToValues(-1, -1, iris, mb)
	if *i != 0 || *m != -150 {
		t.Errorf("bottom-left = %v %v", *i, *m)
	}
	i, m = PadToValues(0, 0, iris, mb)
	if !near(*i, 0.5, 1e-9) || *m != 0 {
		t.Errorf("centre = %v %v", *i, *m)
	}
	i, m = PadToValues(0.5, 0.5, nil, mb)
	if i != nil || m == nil {
		t.Errorf("missing iris must give nil")
	}
	i, m = PadToValues(0.5, 0.5, iris, nil)
	if m != nil || i == nil {
		t.Errorf("missing master black must give nil")
	}
}

func TestValuesToPad(t *testing.T) {
	iris := Range{Min: 0, Max: 1, Step: 0.01}
	mb := Range{Min: -150, Max: 150, Step: 1}
	x, y := ValuesToPad(&mb, &iris, 75, 0.75)
	if !near(x, 0.5, 1e-9) || !near(y, 0.5, 1e-9) {
		t.Errorf("= %v,%v", x, y)
	}
	x, y = ValuesToPad(nil, nil, 0, 0)
	if x != 0 || y != 0 {
		t.Errorf("no axes = %v,%v", x, y)
	}
}

func TestChooseKind(t *testing.T) {
	mk := func(k core.ControlKind, ro bool, n int) core.ControlSpec {
		s := core.ControlSpec{ID: "x", Kind: k, ReadOnly: ro}
		for i := 0; i < n; i++ {
			s.Options = append(s.Options, core.Option{Value: string(rune('a' + i)), Label: "o"})
		}
		return s
	}
	cases := []struct {
		spec core.ControlSpec
		want WidgetKind
	}{
		{mk(core.KindRange, false, 0), KindSlider},
		{mk(core.KindEnum, false, 2), KindSegmented},
		{mk(core.KindEnum, false, 5), KindSegmented},
		{mk(core.KindEnum, false, 6), KindSelect},
		{mk(core.KindEnum, false, 100), KindSelect},
		{mk(core.KindToggle, false, 0), KindCheck},
		{mk(core.KindAction, false, 0), KindButton},
		{mk(core.KindToggle, true, 0), KindLabel},
		{mk(core.KindRange, true, 0), KindLabel},
	}
	for _, c := range cases {
		if got := ChooseKind(c.spec); got != c.want {
			t.Errorf("%s ro=%v n=%d: got %s want %s", c.spec.Kind, c.spec.ReadOnly, len(c.spec.Options), got, c.want)
		}
	}
}

func TestThrottlerCoalescesAndSendsFinalValue(t *testing.T) {
	th := NewThrottler(20 * time.Millisecond)
	var mu sync.Mutex
	var got []int
	start := time.Now()
	for i := 1; i <= 200; i++ {
		i := i
		th.Do("k", func() { mu.Lock(); got = append(got, i); mu.Unlock() })
		time.Sleep(time.Millisecond)
	}
	th.Wait()
	mu.Lock()
	defer mu.Unlock()
	if len(got) == 0 || got[len(got)-1] != 200 {
		t.Fatalf("final value not sent: %v", got)
	}
	if max := int(time.Since(start)/(20*time.Millisecond)) + 2; len(got) > max || len(got) > 150 {
		t.Errorf("not throttled: %d calls for 200 submissions over ~200ms", len(got))
	}
	for i := 1; i < len(got); i++ {
		if got[i] <= got[i-1] {
			t.Fatalf("out of order: %v", got)
		}
	}
}

func TestThrottlerKeysAreIndependentAndSerial(t *testing.T) {
	th := NewThrottler(5 * time.Millisecond)
	var running, maxRun int32
	var a, b int32
	do := func(key string, ctr *int32) {
		th.Do(key, func() {
			n := atomic.AddInt32(&running, 1)
			if n > maxRun {
				atomic.StoreInt32(&maxRun, n)
			}
			time.Sleep(2 * time.Millisecond)
			atomic.AddInt32(ctr, 1)
			atomic.AddInt32(&running, -1)
		})
	}
	do("a", &a)
	do("b", &b)
	th.Wait()
	if a != 1 || b != 1 {
		t.Errorf("each key must run once: a=%d b=%d", a, b)
	}
	// same key never runs concurrently with itself
	var inFlight, overlap int32
	for i := 0; i < 50; i++ {
		th.Do("s", func() {
			if atomic.AddInt32(&inFlight, 1) > 1 {
				atomic.AddInt32(&overlap, 1)
			}
			time.Sleep(time.Millisecond)
			atomic.AddInt32(&inFlight, -1)
		})
	}
	th.Wait()
	if overlap != 0 {
		t.Errorf("same key ran concurrently %d times", overlap)
	}
}
