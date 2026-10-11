// Package texts maps the application's types (controls, groups, errors,
// connection states) to translated text. The wording itself lives in
// internal/i18n/locales; this package only picks the keys.
package texts

import (
	"errors"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/valentindoche/rcp-for-all-camera/internal/app"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/i18n"
)

// Texts translates for one language. It is cheap to copy; a language switch
// builds a new one.
type Texts struct{ *i18n.Translator }

// New wraps a translator.
func New(tr *i18n.Translator) Texts { return Texts{tr} }

// Default is the French texts (tests and fallbacks).
func Default() Texts { return New(i18n.Default().Translator("fr")) }

// ControlLabel is the label of a control: the standard translation, else the
// label the driver sent, else the id.
func (x Texts) ControlLabel(s core.ControlSpec) string {
	if k := "control." + string(s.ID); x.Has(k) {
		return x.T(k)
	}
	if s.Label != "" {
		return s.Label
	}
	return string(s.ID)
}

// GroupOrder is the display order of the generic sections.
var GroupOrder = []core.ControlGroup{
	core.GroupLens, core.GroupExposure, core.GroupWhite, core.GroupColor,
	core.GroupImage, core.GroupPTZ, core.GroupTransport, core.GroupAudio,
	core.GroupOutput, core.GroupSystem, core.GroupMisc,
}

// GroupTitle is the title of a control group; unknown groups read as misc.
func (x Texts) GroupTitle(g core.ControlGroup) string {
	for _, k := range GroupOrder {
		if g == k {
			return x.T("group." + string(g))
		}
	}
	return x.T("group.misc")
}

// SupportLabel is the name of a driver support level.
func (x Texts) SupportLabel(s core.SupportLevel) string {
	switch s {
	case core.SupportPlanned, core.SupportExperimental, core.SupportSupported:
		return x.T("support." + string(s))
	}
	return string(s)
}

// SupportHelp explains a support level in one sentence.
func (x Texts) SupportHelp(s core.SupportLevel) string {
	switch s {
	case core.SupportPlanned, core.SupportExperimental, core.SupportSupported:
		return x.T("support.help." + string(s))
	}
	return ""
}

// ConnLabel is the name of a connection state.
func (x Texts) ConnLabel(s core.ConnState) string {
	switch s {
	case core.StateConnected:
		return x.T("conn.connected")
	case core.StateConnecting:
		return x.T("conn.connecting")
	case core.StateError:
		return x.T("conn.error")
	}
	return x.T("conn.offline")
}

// Decimals is how many decimals to show for a control step.
func Decimals(step float64) int {
	if step <= 0 || math.IsNaN(step) {
		return 2
	}
	for d := 0; d < 4; d++ {
		x := step * math.Pow10(d)
		if math.Abs(x-math.Round(x)) < 1e-9 {
			return d
		}
	}
	return 3
}

// Missing is what an unknown value looks like.
const Missing = "--"

// FormatNumber formats v with the language's decimal mark.
func (x Texts) FormatNumber(v float64, decimals int) string { return x.Number(v, decimals) }

// FormatRate formats a byte rate with the language's units.
func (x Texts) FormatRate(bps float64) string { return x.Rate(bps) }

// FormatMemory formats used / total memory.
func (x Texts) FormatMemory(used, total uint64) string { return x.Memory(used, total) }

// FormatValue renders a control value for display. A nil or mismatching value
// gives Missing.
func (x Texts) FormatValue(s core.ControlSpec, v *core.Value) string {
	if v == nil || v.Kind != s.Kind {
		return Missing
	}
	switch s.Kind {
	case core.KindRange:
		txt := x.Number(v.Number, Decimals(s.Step))
		if s.Min < 0 && v.Number > 0 {
			txt = "+" + txt
		}
		switch s.Unit {
		case "":
			return txt
		case "°":
			return txt + "°"
		}
		return txt + " " + s.Unit
	case core.KindEnum:
		for _, o := range s.Options {
			if o.Value == v.Choice {
				return o.Label
			}
		}
		return v.Choice
	case core.KindToggle:
		if v.On {
			return x.T("value.on")
		}
		return x.T("value.off")
	}
	return Missing
}

var numberRe = regexp.MustCompile(`^\s*([+-]?\d+(?:[.,]\d+)?)\s*[^\d\s]*\s*$`)

// ParseNumber reads a number typed by the user: decimal comma or point, with
// an optional trailing unit.
func ParseNumber(s string) (float64, bool) {
	m := numberRe.FindStringSubmatch(s)
	if m == nil {
		return 0, false
	}
	v, err := strconv.ParseFloat(strings.Replace(m[1], ",", ".", 1), 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return v, true
}

// Level is the severity of a status-bar notice.
type Level int

const (
	LevelInfo Level = iota
	LevelError
)

// Notice is a message for the status bar.
type Notice struct {
	Text  string
	Level Level
}

func apiCode(err error) (code, msg string, ok bool) {
	var ae *app.Error
	if !errors.As(err, &ae) {
		return "", "", false
	}
	return ae.Code, ae.Message, true
}

// NoticeFor maps an error returned by a camera call to a status-bar notice.
func (x Texts) NoticeFor(err error) Notice {
	code, _, ok := apiCode(err)
	if !ok {
		return Notice{x.T("error.internal"), LevelError}
	}
	switch code {
	case app.CodeNotImplemented:
		return Notice{x.T("notice.notImplemented"), LevelInfo}
	case app.CodeInvalidValue:
		return Notice{x.T("notice.invalidValue"), LevelError}
	case app.CodeUnknownControl:
		return Notice{x.T("notice.unknownControl"), LevelError}
	case app.CodeNotConnected:
		return Notice{x.T("notice.notConnected"), LevelError}
	case app.CodeReadOnly:
		return Notice{x.T("notice.readOnly"), LevelError}
	case app.CodeEmptySlot:
		return Notice{x.T("notice.emptySlot"), LevelError}
	case app.CodeInvalidSlot:
		return Notice{x.T("notice.invalidSlot"), LevelError}
	case app.CodeEmptyPreset:
		return Notice{x.T("notice.emptyPreset"), LevelError}
	case app.CodeInvalidPreset:
		return Notice{x.T("notice.invalidPreset"), LevelError}
	case app.CodeConfirmRequired:
		return Notice{x.T("notice.confirmRequired"), LevelInfo}
	}
	return Notice{x.T("error.internal"), LevelError}
}

// ConfigError turns an AssignSlot or ClearSlot error into an inline message
// for the configuration form.
func (x Texts) ConfigError(err error) string {
	code, m, ok := apiCode(err)
	if !ok {
		return x.T("error.internalDot")
	}
	switch code {
	case app.CodeInvalidConfig:
		switch {
		case strings.Contains(m, "host must be"):
			return x.T("config.error.host")
		case strings.Contains(m, "username is required"):
			return x.T("config.error.user")
		case strings.Contains(m, "out of range"):
			return x.T("config.error.port")
		case strings.Contains(m, "name must be"):
			return x.T("config.error.name")
		case strings.Contains(m, "transport"):
			return x.T("config.error.transport")
		case strings.Contains(m, "credentials must"):
			return x.T("config.error.credentials")
		}
		return x.T("config.error.invalid")
	case app.CodeUnknownModel:
		return x.T("config.error.model")
	case app.CodeInvalidSlot:
		return x.T("config.error.slot")
	}
	return x.T("error.internalDot")
}

// AutoStartError is the inline message for a failed SetAutoStart.
func (x Texts) AutoStartError(err error) string {
	if code, _, ok := apiCode(err); ok && code == app.CodeNotImplemented {
		return x.T("config.error.autostart.unavailable")
	}
	return x.T("config.error.autostart.failed")
}

// BindError is the inline message for a failed SetBindIP.
func (x Texts) BindError(err error) string {
	if code, _, ok := apiCode(err); ok && code == app.CodeInvalidConfig {
		return x.T("config.error.bind.invalid")
	}
	return x.T("config.error.bind.failed")
}

// ShortLabel trims a long option label for a compact readout: the part
// before the first " (" ("ND 1.2 (4 stops)" gives "ND 1.2").
func ShortLabel(s string) string {
	if i := strings.Index(s, " ("); i > 0 {
		return s[:i]
	}
	return s
}
