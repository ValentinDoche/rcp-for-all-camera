package widgets

import (
	"math"
	"slices"
	"testing"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

var fIris = core.ControlSpec{ID: core.Iris, Kind: core.KindRange, Min: 1, Max: 22, Step: 0.1, Unit: "f"}

func TestIrisScaleUsesTheLensLimits(t *testing.T) {
	k := NewIrisScale(fIris, core.Lens{IrisMin: 2.8, IrisMax: 22})
	if k.Min != 2.8 || k.Max != 22 || !k.FStop {
		t.Fatalf("scale %+v", k)
	}
	if k := NewIrisScale(fIris, core.Lens{}); k.Min != 1 || k.Max != 22 {
		t.Errorf("unknown lens keeps the spec range: %+v", k)
	}
	if k := NewIrisScale(fIris, core.Lens{IrisMin: 30, IrisMax: 40}); k.Min != 1 {
		t.Errorf("a lens outside the spec is ignored: %+v", k)
	}
}

func TestIrisScaleIsLogarithmicAndOpenAtTheSmallestNumber(t *testing.T) {
	k := NewIrisScale(fIris, core.Lens{IrisMin: 2.8, IrisMax: 22})
	if k.Open(2.8) != 1 || k.Open(22) != 0 {
		t.Errorf("open(2.8) = %v, open(22) = %v", k.Open(2.8), k.Open(22))
	}
	if k.At(1) != 2.8 || k.At(0) != 22 {
		t.Errorf("at(1) = %v, at(0) = %v", k.At(1), k.At(0))
	}
	// f/8 is 3 stops from f/22 and 3 stops from f/2.8: the middle.
	if o := k.Open(7.85); math.Abs(o-0.5) > 0.01 {
		t.Errorf("open(7.85) = %v, want 0.5", o)
	}
}

func TestIrisStepByThirdsAndFullStops(t *testing.T) {
	k := NewIrisScale(fIris, core.Lens{IrisMin: 2.8, IrisMax: 22})
	for _, c := range []struct {
		v    float64
		n    int
		want float64
	}{
		{4, 1, 3.5}, {4, -1, 4.5}, {4, 3, 2.8}, {4, -3, 5.6},
		{2.8, 1, 2.8}, {22, -3, 22}, {4.1, 0, 4}, {11, -3, 16},
	} {
		if got := k.StepBy(c.v, c.n); got != c.want {
			t.Errorf("step(%v, %d) = %v, want %v", c.v, c.n, got, c.want)
		}
	}
}

func TestIrisStepOfANormalisedIris(t *testing.T) {
	k := NewIrisScale(core.ControlSpec{ID: core.Iris, Kind: core.KindRange, Min: 0, Max: 1, Step: 0.01}, core.Lens{})
	if k.FStop || k.StepBy(0.5, 3) != 0.6 || k.Open(1) != 1 {
		t.Errorf("linear scale %+v: step %v", k, k.StepBy(0.5, 3))
	}
}

func TestIrisLabels(t *testing.T) {
	k := NewIrisScale(fIris, core.Lens{IrisMin: 2.8, IrisMax: 22})
	if got := k.Labels(); !slices.Equal(got, []float64{2.8, 4, 5.6, 8, 11, 16, 22}) {
		t.Errorf("labels %v", got)
	}
}
