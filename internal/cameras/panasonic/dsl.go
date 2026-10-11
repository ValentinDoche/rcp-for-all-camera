package panasonic

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/speckit"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// The SKAARHOJ parameter lists are transcribed as compact text rows, one
// parameter per line, so each row stays close to the source table and the
// numeric option codes of the Panasonic protocol are kept for later:
//
//	i|SkaarhojID|Label|group|Section|min|max|unit              integer range, step 1
//	f|SkaarhojID|Label|group|Section|min|max|step|unit         float range
//	o|SkaarhojID|Label|group|Section|0:Off;1:On                option list, "code:label"
//	t|SkaarhojID|Label|group|Section                           one-shot trigger
//
// The id is "panasonic." plus the lowerCamel form of SkaarhojID, or the part
// after "=" when SkaarhojID is written "SkaarhojID=customName". An option list
// that is exactly Off/On becomes a toggle. Option values are lowercase slugs of
// the label (the code is not part of core.Option), with the code appended when
// two labels of one list collide.

var groups = map[string]core.ControlGroup{
	"lens": core.GroupLens, "exposure": core.GroupExposure, "white": core.GroupWhite,
	"color": core.GroupColor, "image": core.GroupImage, "ptz": core.GroupPTZ,
	"audio": core.GroupAudio, "output": core.GroupOutput, "system": core.GroupSystem,
}

// parseRows turns the text rows into specs. Static tables: a malformed row is
// a programming error and panics (caught by the contract test).
func parseRows(src string) []core.ControlSpec {
	var out []core.ControlSpec
	for _, line := range strings.Split(src, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, parseRow(line))
	}
	return out
}

func parseRow(line string) core.ControlSpec {
	f := strings.Split(line, "|")
	bad := func(why string) string { return fmt.Sprintf("panasonic: bad row %q: %s", line, why) }
	if len(f) < 5 {
		panic(bad("too few fields"))
	}
	g, ok := groups[f[3]]
	if !ok {
		panic(bad("unknown group"))
	}
	name, custom, _ := strings.Cut(f[1], "=")
	if custom == "" {
		custom = lowerCamel(name)
	}
	s := core.ControlSpec{ID: core.ControlID("panasonic." + custom), Label: f[2], Group: g, Section: f[4]}
	num := func(i int) float64 {
		if i >= len(f) {
			panic(bad("missing number"))
		}
		v, err := strconv.ParseFloat(f[i], 64)
		if err != nil {
			panic(bad("bad number"))
		}
		return v
	}
	unit := func(i int) string {
		if i < len(f) {
			return f[i]
		}
		return ""
	}
	switch f[0] {
	case "i":
		s.Kind, s.Min, s.Max, s.Step, s.Unit = core.KindRange, num(5), num(6), 1, unit(7)
	case "f":
		s.Kind, s.Min, s.Max, s.Step, s.Unit = core.KindRange, num(5), num(6), num(7), unit(8)
	case "t":
		s.Kind = core.KindAction
	case "o":
		if len(f) != 6 {
			panic(bad("option list needs one options field"))
		}
		if f[5] == "0:Off;1:On" {
			s.Kind = core.KindToggle
		} else {
			s.Kind, s.Options = core.KindEnum, parseOptions(f[5])
		}
	default:
		panic(bad("unknown kind"))
	}
	return s
}

func parseOptions(src string) []core.Option {
	var out []core.Option
	seen := map[string]bool{}
	for _, item := range strings.Split(src, ";") {
		code, label, ok := strings.Cut(item, ":")
		if !ok {
			panic("panasonic: bad option " + item)
		}
		v := slug(label)
		if seen[v] {
			v += "-" + code
			label += " (" + code + ")"
		}
		seen[v] = true
		out = append(out, core.Option{Value: v, Label: label})
	}
	return out
}

var (
	negDigit = regexp.MustCompile(`(^|\s)-(\d)`)
	nonSlug  = regexp.MustCompile(`[^a-z0-9.]+`)
)

// slug makes a stable lowercase option value from a displayed label.
func slug(label string) string {
	s := strings.ReplaceAll(label, "+", " plus ")
	s = negDigit.ReplaceAllString(s, "${1}minus ${2}")
	s = nonSlug.ReplaceAllString(strings.ToLower(s), "-")
	return strings.Trim(s, "-")
}

// lowerCamel lowercases the leading capital letters of a SKAARHOJ id:
// "KneePointMaster" -> "kneePointMaster", "AWBExec" -> "awbExec",
// "RGamma" -> "rGamma". Ids that start with a digit are kept as they are.
func lowerCamel(id string) string {
	n := 0
	for n < len(id) && id[n] >= 'A' && id[n] <= 'Z' {
		n++
	}
	switch {
	case n == 0:
		return id
	case n == len(id) || n == 1:
		return strings.ToLower(id[:n]) + id[n:]
	case id[n] >= 'a' && id[n] <= 'z':
		return strings.ToLower(id[:n-1]) + id[n-1:]
	default: // capitals followed by a digit, '_' ...
		return strings.ToLower(id[:n]) + id[n:]
	}
}

// presetOptions builds "Preset 1".."Preset n" with values "1".."n".
func presetOptions(n int) []core.Option {
	out := make([]core.Option, n)
	for i := range out {
		v := strconv.Itoa(i + 1)
		out[i] = core.Option{Value: v, Label: "Preset " + v}
	}
	return out
}

// gainOptions builds the GainControl list: one option per dB step from lo to
// hi (value "-6", "0", "12"...) followed by Auto.
func gainOptions(lo, hi int) []core.Option {
	out := make([]core.Option, 0, hi-lo+2)
	for db := lo; db <= hi; db++ {
		v := strconv.Itoa(db)
		out = append(out, core.Option{Value: v, Label: v + " dB"})
	}
	return append(out, core.Option{Value: "auto", Label: "Auto"})
}

// section tags a spec with a section and returns it.
func section(s core.ControlSpec, name string) core.ControlSpec {
	s.Section = name
	return s
}

// stdParams holds what differs between models in the standard controls.
type stdParams struct {
	gainLo, gainHi        int      // GainControl dB steps (code = dB + 8)
	shutter               []string // ShutterSpeed values
	pedestal, pedestalRGB float64  // MasterPedestal / PedestalR,G,B symmetric limit
	colorGain             float64  // ColorGainR/B(/G) symmetric limit
	chromaMin, chromaMax  float64  // ChromaLevel (%)
	greenColorGain        bool     // ColorGainG exists
}

// standardControls builds the controls that map onto standard ids.
func standardControls(p stdParams) []core.ControlSpec {
	s := []core.ControlSpec{
		// CamIrisFvalue, F1.4..F25.5. The raw CamIris / IrisPosition scales are not exposed.
		section(speckit.Range(core.Iris, 1.4, 25.5, 0.1, "F"), "Iris"),
		// FocusPosition raw scale 1365..4095, normalized to 0..1 by the driver.
		section(speckit.Range(core.Focus, 0, 1, 0.001, ""), "Focus"),
		section(speckit.Action(core.AutoFocus), "Focus"), // CamFocusOneTouch
		// ZoomPosition raw scale 1365..4095, normalized to 0..1 by the driver.
		section(speckit.Range(core.Zoom, 0, 1, 0.001, ""), "Zoom"),
		section(speckit.Enum(core.ND, speckit.Labeled("clear", "Through", "1/4", "1/4", "1/16", "1/16", "1/64", "1/64")), "ND filter"),
		section(speckit.Enum(core.Gain, gainOptions(p.gainLo, p.gainHi)), "Gain"),
		section(speckit.Enum(core.ShutterSpeed, speckit.Opts(p.shutter...)), "Shutter"),
		section(speckit.Range(core.WhiteBalance, 2000, 15000, 100, "K"), "AWB"), // ColorTemperature
		section(speckit.Action(core.AutoWhiteBalance), "AWB"),                   // AWBExec
		section(speckit.Range(core.MasterBlack, -p.pedestal, p.pedestal, 1, ""), "Pedestal"),
		section(speckit.Range(core.LiftR, -p.pedestalRGB, p.pedestalRGB, 1, ""), "Pedestal"),
		section(speckit.Range(core.LiftG, -p.pedestalRGB, p.pedestalRGB, 1, ""), "Pedestal"),
		section(speckit.Range(core.LiftB, -p.pedestalRGB, p.pedestalRGB, 1, ""), "Pedestal"),
		section(speckit.Range(core.GainR, -p.colorGain, p.colorGain, 1, ""), "Color gain"),
		section(speckit.Range(core.GainB, -p.colorGain, p.colorGain, 1, ""), "Color gain"),
		section(speckit.Range(core.Saturation, p.chromaMin, p.chromaMax, 1, "%"), "Chroma"), // ChromaLevel
		section(speckit.Range(core.Detail, -31, 31, 1, ""), "Detail"),                       // TotalDtlLevel
		// ColorBar: On shows the bars (UE150 labels it Camera / Color Bars).
		section(speckit.Toggle(core.ColorBars), "Color bars"),
		speckit.Range(core.Pan, -1, 1, 0.01, ""),
		speckit.Range(core.Tilt, -1, 1, 0.01, ""),
		section(speckit.Enum(core.PresetRecall, presetOptions(100)), "Presets"),
	}
	if p.greenColorGain {
		s = append(s, section(speckit.Range(core.GainG, -p.colorGain, p.colorGain, 1, ""), "Color gain"))
	}
	return s
}

// colorCorrectRows builds the 12/16 axis phase and saturation rows of the
// color correction matrix. parts are {id fragment, label}.
func colorCorrectRows(parts [][2]string, min, max int) string {
	var b strings.Builder
	for _, p := range parts {
		fmt.Fprintf(&b, "i|ColorCorrect%sPhase|%s phase|color|Matrix phase|%d|%d|\n", p[0], p[1], min, max)
	}
	for _, p := range parts {
		fmt.Fprintf(&b, "i|ColorCorrect%sGain|%s saturation|color|Matrix saturation|%d|%d|\n", p[0], p[1], min, max)
	}
	return b.String()
}

// presetActionRows are the Presets rows shared by both models.
const presetActionRows = `
t|presetHome|Home position|ptz|Presets
`
