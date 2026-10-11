package widgets

import (
	"math"
	"slices"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// IrisScale maps an iris value to how open the iris is (0 closed, 1 fully
// open) and back. An iris in f-stops (unit "f") is logarithmic, the smallest
// f-number being the most open, and is narrowed to the lens' own limits when
// the camera reports them. Any other iris is linear, Max being open.
type IrisScale struct {
	Min, Max, Step float64
	FStop          bool
}

// NewIrisScale builds the scale of an iris control for the mounted lens.
func NewIrisScale(s core.ControlSpec, l core.Lens) IrisScale {
	k := IrisScale{Min: s.Min, Max: s.Max, Step: s.Step, FStop: s.Unit == "f"}
	if k.FStop && l.IrisMin > 0 && l.IrisMax >= l.IrisMin {
		lo, hi := math.Max(k.Min, l.IrisMin), math.Min(k.Max, l.IrisMax)
		if lo <= hi {
			k.Min, k.Max = lo, hi
		}
	}
	return k
}

func clamp01(v float64) float64 { return math.Max(0, math.Min(1, v)) }

func (k IrisScale) logarithmic() bool { return k.FStop && k.Min > 0 && k.Max > k.Min }

// Open is how open v is, 0..1.
func (k IrisScale) Open(v float64) float64 {
	switch {
	case k.Max <= k.Min:
		return 0
	case k.logarithmic():
		return clamp01(math.Log(k.Max/v) / math.Log(k.Max/k.Min))
	}
	return clamp01((v - k.Min) / (k.Max - k.Min))
}

// At is the iris value for an openness 0..1, snapped to the step.
func (k IrisScale) At(open float64) float64 {
	open = clamp01(open)
	r := Range{Min: k.Min, Max: k.Max, Step: k.Step}
	if k.logarithmic() {
		return Quantize(k.Max*math.Pow(k.Min/k.Max, open), r)
	}
	return Quantize(k.Min+open*(k.Max-k.Min), r)
}

// ThirdStops are the marked f-stops in third-stop increments.
var ThirdStops = []float64{
	1, 1.1, 1.2, 1.4, 1.6, 1.8, 2, 2.2, 2.5, 2.8, 3.2, 3.5, 4, 4.5, 5, 5.6,
	6.3, 7.1, 8, 9, 10, 11, 13, 14, 16, 18, 20, 22, 25, 29, 32,
}

// FullStops are the full f-stops, labelled on the iris scale.
var FullStops = []float64{1, 1.4, 2, 2.8, 4, 5.6, 8, 11, 16, 22, 32}

// stops are the third stops inside the scale, with its ends.
func (k IrisScale) stops() []float64 {
	out := []float64{k.Min, k.Max}
	for _, f := range ThirdStops {
		if f > k.Min*1.01 && f < k.Max*0.99 {
			out = append(out, f)
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// StepBy moves v by n third stops; n > 0 opens the iris. An f-stop iris lands
// on the marked third stops, a linear iris moves by a thirtieth of its range.
func (k IrisScale) StepBy(v float64, n int) float64 {
	if !k.FStop {
		r := Range{Min: k.Min, Max: k.Max, Step: k.Step}
		return Quantize(v+float64(n)*(k.Max-k.Min)/30, r)
	}
	st := k.stops()
	i := 0
	for j, f := range st { // nearest stop, by ratio
		if math.Abs(math.Log(f/v)) < math.Abs(math.Log(st[i]/v)) {
			i = j
		}
	}
	i = max(0, min(len(st)-1, i-n))
	return st[i]
}

// Labels are the full stops inside the scale, for the scale under a bar.
func (k IrisScale) Labels() []float64 {
	if !k.FStop {
		return nil
	}
	var out []float64
	for _, f := range FullStops {
		if f >= k.Min*0.99 && f <= k.Max*1.01 {
			out = append(out, f)
		}
	}
	return out
}
