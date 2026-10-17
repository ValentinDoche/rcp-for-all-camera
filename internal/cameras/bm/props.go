package bm

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// Blackmagic-specific control ids: the cameras expose them, no standard id
// fits. Their "blackmagic." prefix is kept from 0.2.x: presets store them.
const (
	OffsetR          core.ControlID = "blackmagic.offsetR"
	OffsetG          core.ControlID = "blackmagic.offsetG"
	OffsetB          core.ControlID = "blackmagic.offsetB"
	OffsetY          core.ControlID = "blackmagic.offsetY"
	ContrastPivot    core.ControlID = "blackmagic.contrastPivot"
	LumaContribution core.ControlID = "blackmagic.lumaContribution"
)

const (
	pathFormat   = "/system/format"
	pathIris     = "/lens/iris"
	pathFocus    = "/lens/focus"
	pathZoom     = "/lens/zoom"
	pathIrisDesc = "/lens/iris/description"
	pathFocDesc  = "/lens/focus/description"
	pathZoomDesc = "/lens/zoom/description"
	pathGain     = "/video/gain"
	pathISO      = "/video/iso"
	pathShutter  = "/video/shutter"
	pathND       = "/video/ndFilter"
	pathWB       = "/video/whiteBalance"
	pathTint     = "/video/whiteBalanceTint"
	pathAutoExp  = "/video/autoExposure"
	pathAutoWB   = "/video/whiteBalance/doAuto"
	pathAutoFoc  = "/lens/focus/doAutoFocus"
	pathRecord   = "/transports/0/record"
	pathLift     = "/colorCorrection/lift"
	pathGamma    = "/colorCorrection/gamma"
	pathGainCC   = "/colorCorrection/gain"
	pathOffset   = "/colorCorrection/offset"
	pathContrast = "/colorCorrection/contrast"
	pathColor    = "/colorCorrection/color"
	pathLuma     = "/colorCorrection/lumaContribution"
)

// watched are the properties the driver subscribes to on the event socket.
// The camera answers the subscription with their current values.
var watched = []string{
	pathFormat, pathIris, pathFocus, pathZoom, pathGain, pathISO, pathShutter, pathND,
	pathWB, pathTint, pathRecord, pathLift, pathGamma, pathGainCC, pathOffset,
	pathContrast, pathColor, pathLuma,
}

// lensWatched are the lens descriptions, subscribed to apart from watched:
// some firmwares (seen on Studio Camera 6K Pro) do not know them and
// refuse any subscription that names them.
var lensWatched = []string{pathIrisDesc, pathFocDesc, pathZoomDesc}

// colorCtl maps one control onto one key of a color correction property.
type colorCtl struct {
	id   core.ControlID
	path string
	key  string
}

// colorCtls is the single source of truth for the color correction controls,
// used both to decode what the camera reports and to build what we send.
var colorCtls = func() []colorCtl {
	var t []colorCtl
	ch := []string{"red", "green", "blue", "luma"}
	for _, g := range []struct {
		path string
		ids  [4]core.ControlID
	}{
		{pathLift, [4]core.ControlID{core.LiftR, core.LiftG, core.LiftB, core.LiftY}},
		{pathGamma, [4]core.ControlID{core.GammaR, core.GammaG, core.GammaB, core.GammaY}},
		{pathGainCC, [4]core.ControlID{core.GainR, core.GainG, core.GainB, core.GainY}},
		{pathOffset, [4]core.ControlID{OffsetR, OffsetG, OffsetB, OffsetY}},
	} {
		for i, id := range g.ids {
			t = append(t, colorCtl{id, g.path, ch[i]})
		}
	}
	return append(t,
		// TODO(verify): the panel's master black ring is lift luma on ATEM cameras.
		colorCtl{core.MasterBlack, pathLift, "luma"},
		colorCtl{core.Contrast, pathContrast, "adjust"},
		colorCtl{ContrastPivot, pathContrast, "pivot"},
		colorCtl{core.Saturation, pathColor, "saturation"},
		colorCtl{core.Hue, pathColor, "hue"},
		colorCtl{LumaContribution, pathLuma, "lumaContribution"},
	)
}()

// decoder turns the JSON object of one property into control values. It runs
// with the camera lock held and must not call back into the camera.
type decoder func(c *Camera, o map[string]any) map[core.ControlID]core.Value

var decoders = func() map[string]decoder {
	m := map[string]decoder{
		pathFormat:  decodeFormat,
		pathIris:    decodeIris,
		pathFocus:   numberOf(core.Focus, "normalised", "focus"),
		pathZoom:    numberOf(core.Zoom, "normalised"),
		pathGain:    numberOf(core.Gain, "gain"),
		pathWB:      numberOf(core.WhiteBalance, "whiteBalance"),
		pathTint:    numberOf(core.Tint, "whiteBalanceTint"),
		pathISO:     decodeISO,
		pathND:      decodeND,
		pathShutter: decodeShutter,
		pathRecord:  decodeRecord,
	}
	for _, cc := range colorCtls {
		if _, ok := m[cc.path]; !ok {
			m[cc.path] = decodeColor(cc.path)
		}
	}
	return m
}()

// idPath is the property that holds each readable control.
var idPath = func() map[core.ControlID]string {
	m := map[core.ControlID]string{
		core.Iris: pathIris, core.IrisNorm: pathIris, core.Focus: pathFocus, core.Zoom: pathZoom, core.Gain: pathGain,
		core.ISO: pathISO, core.ShutterSpeed: pathShutter, core.ShutterAngle: pathShutter,
		core.ND: pathND, core.WhiteBalance: pathWB, core.Tint: pathTint, core.Record: pathRecord,
	}
	for _, cc := range colorCtls {
		m[cc.id] = cc.path
	}
	return m
}()

// num reads a number. The camera stores float32, so 0.1 comes back as
// 0.10000000149; rounding to 6 decimals gives back the value that was sent.
func num(o map[string]any, key string) (float64, bool) {
	f, ok := o[key].(float64)
	if !ok || math.IsNaN(f) || math.IsInf(f, 0) {
		return 0, false
	}
	return math.Round(f*1e6) / 1e6, true
}

// numberOf reads the first of keys that is present.
func numberOf(id core.ControlID, keys ...string) decoder {
	return func(_ *Camera, o map[string]any) map[core.ControlID]core.Value {
		for _, k := range keys {
			if f, ok := num(o, k); ok {
				return map[core.ControlID]core.Value{id: core.Number(f)}
			}
		}
		return nil
	}
}

// decodeIris reads the f-stop (Iris) and the position over the lens' range
// (IrisNorm) of /lens/iris.
func decodeIris(_ *Camera, o map[string]any) map[core.ControlID]core.Value {
	out := map[core.ControlID]core.Value{}
	if f, ok := num(o, "apertureStop"); ok {
		out[core.Iris] = core.Number(f)
	}
	if n, ok := num(o, "normalised"); ok {
		out[core.IrisNorm] = core.Number(n)
	}
	return out
}

// irisEndTol is how close to 0 or 1 "normalised" must be to mean the lens'
// fully open or closed end.
const irisEndTol = 0.002

// learnIrisEnds infers the lens' f-stop limits from /lens/iris for firmwares
// without /lens/iris/description: "normalised" is 0 at the lens' open end
// and 1 at its closed end, so the f-stop reported there is that limit. A
// stop reported beyond a learned limit (another lens) forgets that limit.
// It runs with the camera lock held; c.irisDesc means the camera reported
// real limits, which are kept.
func (c *Camera) learnIrisEnds(o map[string]any) {
	if c.irisDesc {
		return
	}
	stop, ok := num(o, "apertureStop")
	if !ok || stop <= 0 {
		return
	}
	n, hasN := num(o, "normalised")
	switch {
	case hasN && n <= irisEndTol:
		c.lens.IrisMin = stop
	case hasN && n >= 1-irisEndTol:
		c.lens.IrisMax = stop
	default:
		if c.lens.IrisMin > 0 && stop < c.lens.IrisMin {
			c.lens.IrisMin = 0
		}
		if c.lens.IrisMax > 0 && stop > c.lens.IrisMax {
			c.lens.IrisMax = 0
		}
	}
}

func decodeColor(path string) decoder {
	return func(_ *Camera, o map[string]any) map[core.ControlID]core.Value {
		out := map[core.ControlID]core.Value{}
		for _, cc := range colorCtls {
			if cc.path != path {
				continue
			}
			if f, ok := num(o, cc.key); ok {
				out[cc.id] = core.Number(f)
			}
		}
		return out
	}
}

func decodeISO(_ *Camera, o map[string]any) map[core.ControlID]core.Value {
	if f, ok := num(o, "iso"); ok {
		return map[core.ControlID]core.Value{core.ISO: core.Choice(strconv.Itoa(int(math.Round(f))))}
	}
	return nil
}

func decodeND(_ *Camera, o map[string]any) map[core.ControlID]core.Value {
	if f, ok := num(o, "stop"); ok {
		return map[core.ControlID]core.Value{core.ND: core.Choice(strconv.FormatFloat(math.Round(f*100)/100, 'f', -1, 64))}
	}
	return nil
}

func decodeRecord(_ *Camera, o map[string]any) map[core.ControlID]core.Value {
	if b, ok := o["recording"].(bool); ok {
		return map[core.ControlID]core.Value{core.Record: core.Bool(b)}
	}
	return nil
}

// decodeFormat only records the frame rate, which the shutter conversion needs;
// it then re-decodes the last shutter reading with it.
func decodeFormat(c *Camera, o map[string]any) map[core.ControlID]core.Value {
	fps := 0.0
	if on, _ := o["offSpeedEnabled"].(bool); on {
		fps, _ = num(o, "offSpeedFrameRate")
	} else if s, ok := o["frameRate"].(string); ok {
		fps, _ = strconv.ParseFloat(s, 64)
	}
	c.fps = fps
	if sh := c.raw[pathShutter]; sh != nil {
		return decodeShutter(c, sh)
	}
	return nil
}

// decodeShutter exposes both the speed and the angle, whichever the camera is
// set to report, converting with the frame rate: angle = 360 * fps / speed.
func decodeShutter(c *Camera, o map[string]any) map[core.ControlID]core.Value {
	speed, hasSpeed := num(o, "shutterSpeed")
	angle, hasAngle := num(o, "shutterAngle")
	hasSpeed = hasSpeed && speed > 0
	hasAngle = hasAngle && angle > 0
	angle /= 100 // the API sends hundredths of a degree
	switch {
	case hasSpeed && !hasAngle && c.fps > 0:
		angle, hasAngle = 360*c.fps/speed, true
	case hasAngle && !hasSpeed && c.fps > 0:
		speed, hasSpeed = 360*c.fps/angle, true
	}
	out := map[core.ControlID]core.Value{}
	if hasSpeed {
		out[core.ShutterSpeed] = core.Choice("1/" + strconv.Itoa(int(math.Round(speed))))
	}
	if hasAngle {
		out[core.ShutterAngle] = core.Number(math.Round(angle*10) / 10)
	}
	return out
}

func kv(pairs ...any) map[string]any {
	m := make(map[string]any, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		m[pairs[i].(string)] = pairs[i+1]
	}
	return m
}

// commandFor builds the REST call for a validated value.
func commandFor(id core.ControlID, v core.Value) (Command, error) {
	switch id {
	case core.Iris:
		return Command{Path: pathIris, Body: kv("apertureStop", v.Number)}, nil
	case core.IrisNorm:
		return Command{Path: pathIris, Body: kv("normalised", v.Number)}, nil
	case core.AutoIris:
		return Command{Path: pathAutoExp, Body: kv("mode", "OneShot", "type", "Iris")}, nil
	case core.Focus:
		return Command{Path: pathFocus, Body: kv("normalised", v.Number)}, nil
	case core.AutoFocus:
		return Command{Path: pathAutoFoc, Body: kv()}, nil
	case core.Zoom:
		return Command{Path: pathZoom, Body: kv("normalised", v.Number)}, nil
	case core.Gain:
		return Command{Path: pathGain, Body: kv("gain", int(math.Round(v.Number)))}, nil
	case core.ISO:
		n, err := strconv.Atoi(v.Choice)
		if err != nil {
			return Command{}, fmt.Errorf("%s: bad ISO: %w", id, core.ErrInvalidValue)
		}
		return Command{Path: pathISO, Body: kv("iso", n)}, nil
	case core.ShutterSpeed:
		n, err := strconv.Atoi(strings.TrimPrefix(v.Choice, "1/"))
		if err != nil || n <= 0 {
			return Command{}, fmt.Errorf("%s: bad shutter speed: %w", id, core.ErrInvalidValue)
		}
		return Command{Path: pathShutter, Body: kv("shutterSpeed", n)}, nil
	case core.ShutterAngle:
		return Command{Path: pathShutter, Body: kv("shutterAngle", int(math.Round(v.Number*100)))}, nil
	case core.ND:
		f, err := strconv.ParseFloat(v.Choice, 64)
		if err != nil {
			return Command{}, fmt.Errorf("%s: bad ND stop: %w", id, core.ErrInvalidValue)
		}
		return Command{Path: pathND, Body: kv("stop", f)}, nil
	case core.WhiteBalance:
		return Command{Path: pathWB, Body: kv("whiteBalance", int(math.Round(v.Number)))}, nil
	case core.Tint:
		return Command{Path: pathTint, Body: kv("whiteBalanceTint", int(math.Round(v.Number)))}, nil
	case core.AutoWhiteBalance:
		return Command{Path: pathAutoWB, Body: kv()}, nil
	case core.Record:
		return Command{Path: pathRecord, Body: kv("recording", v.On)}, nil
	}
	for _, cc := range colorCtls {
		if cc.id == id {
			return Command{Path: cc.path, Body: kv(cc.key, v.Number), Composite: true}, nil
		}
	}
	return Command{}, fmt.Errorf("%s: %w", id, core.ErrUnknownControl)
}

// lensDecoders update the lens information from the lens properties. They run
// with the camera lock held, like decoders.
var lensDecoders = map[string]func(l *core.Lens, o map[string]any){
	pathZoom: func(l *core.Lens, o map[string]any) {
		if f, ok := num(o, "focalLength"); ok && f > 0 {
			l.FocalLength = f
		}
	},
	// The documented GET has no distance, but the PUT takes focusDistance;
	// read it when a firmware reports it.
	pathFocus: func(l *core.Lens, o map[string]any) {
		if f, ok := num(o, "focusDistance"); ok && f > 0 {
			l.FocusDistance = focusMetres(f)
		}
	},
	pathIrisDesc: func(l *core.Lens, o map[string]any) {
		l.IrisMin, l.IrisMax = minMax(o, "apertureStop")
	},
	pathZoomDesc: func(l *core.Lens, o map[string]any) {
		l.FocalMin, l.FocalMax = minMax(o, "focalLength")
	},
	pathFocDesc: func(l *core.Lens, o map[string]any) {
		near, far := minMax(o, "focusDistance")
		l.FocusNear, l.FocusFar = focusMetres(near), focusMetres(far)
	},
}

// minMax reads {"key": {"min": a, "max": b}}; a missing or bad pair is 0, 0.
func minMax(o map[string]any, key string) (lo, hi float64) {
	m, _ := o[key].(map[string]any)
	lo, okLo := num(m, "min")
	hi, okHi := num(m, "max")
	if !okLo || !okHi || lo <= 0 || hi < lo {
		return 0, 0
	}
	return lo, hi
}

// focusInfinityMM is the distance from which the camera means infinity.
// TODO(verify): the API documents no unit nor infinity encoding for focus
// distances; millimetres are assumed, as for the integer focusDistance PUT.
const focusInfinityMM = 1e6

// focusMetres converts a focus distance from the camera (millimetres) to
// metres; 0 stays unknown.
func focusMetres(mm float64) float64 {
	switch {
	case mm <= 0:
		return 0
	case mm >= focusInfinityMM:
		return core.FocusInfinity
	}
	return mm / 1000
}
