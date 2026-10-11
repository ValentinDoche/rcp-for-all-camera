// Package fr holds every French string, label mapping and number format of
// the GUI, so the wording lives in one place and is testable without Fyne.
package fr

import (
	"errors"
	"math"
	"regexp"
	"strconv"
	"strings"

	"github.com/valentindoche/rcp-for-all-camera/internal/app"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// labels maps the standard control IDs to French labels. Unknown IDs fall
// back to the spec's own label.
var labels = map[core.ControlID]string{
	core.Iris:             "Iris",
	core.AutoIris:         "Iris auto",
	core.Focus:            "Mise au point",
	core.AutoFocus:        "Mise au point auto",
	core.Zoom:             "Zoom",
	core.Gain:             "Gain",
	core.ISO:              "ISO",
	core.ShutterAngle:     "Angle d'obturateur",
	core.ShutterSpeed:     "Vitesse d'obturateur",
	core.ND:               "Filtre ND",
	core.MasterBlack:      "Noir maître",
	core.WhiteBalance:     "Balance des blancs",
	core.Tint:             "Teinte",
	core.AutoWhiteBalance: "Balance des blancs auto",
	core.LiftR:            "Lift rouge",
	core.LiftG:            "Lift vert",
	core.LiftB:            "Lift bleu",
	core.LiftY:            "Lift luminance",
	core.GammaR:           "Gamma rouge",
	core.GammaG:           "Gamma vert",
	core.GammaB:           "Gamma bleu",
	core.GammaY:           "Gamma luminance",
	core.GainR:            "Gain rouge",
	core.GainG:            "Gain vert",
	core.GainB:            "Gain bleu",
	core.GainY:            "Gain luminance",
	core.Contrast:         "Contraste",
	core.Saturation:       "Saturation",
	core.Hue:              "Teinte (hue)",
	core.Detail:           "Détail",
	core.ColorBars:        "Mire de barres",
	core.Pan:              "Panoramique",
	core.Tilt:             "Inclinaison",
	core.PresetRecall:     "Preset",
	core.Record:           "Enregistrement",
	core.Tally:            "Tally",
}

// ControlLabel is the French label of a control.
func ControlLabel(s core.ControlSpec) string {
	if l, ok := labels[s.ID]; ok {
		return l
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

// GroupTitle is the French title of a control group.
func GroupTitle(g core.ControlGroup) string {
	switch g {
	case core.GroupLens:
		return "Objectif"
	case core.GroupExposure:
		return "Exposition"
	case core.GroupWhite:
		return "Balance des blancs"
	case core.GroupColor:
		return "Correction colorimétrique"
	case core.GroupImage:
		return "Image"
	case core.GroupPTZ:
		return "PTZ"
	case core.GroupTransport:
		return "Enregistrement"
	case core.GroupAudio:
		return "Audio"
	case core.GroupOutput:
		return "Sorties vidéo"
	case core.GroupSystem:
		return "Système"
	}
	return "Divers"
}

// TransportLabel is the French name of a link type.
func TransportLabel(t core.Transport) string {
	switch t {
	case core.TransportEthernet:
		return "Ethernet"
	case core.TransportWiFi:
		return "Wi-Fi"
	case core.TransportUSBEthernet:
		return "Adaptateur USB-C Ethernet"
	}
	return string(t)
}

// SupportLabel is the French name of a driver support level.
func SupportLabel(s core.SupportLevel) string {
	switch s {
	case core.SupportPlanned:
		return "Prévu"
	case core.SupportExperimental:
		return "Expérimental"
	case core.SupportSupported:
		return "Pris en charge"
	}
	return string(s)
}

// SupportHelp explains a support level in one sentence.
func SupportHelp(s core.SupportLevel) string {
	switch s {
	case core.SupportPlanned:
		return "Pilote en développement : les contrôles sont déclarés mais pas encore envoyés à la caméra."
	case core.SupportExperimental:
		return "Pilote fonctionnel, pas encore vérifié sur une caméra réelle."
	case core.SupportSupported:
		return "Pilote vérifié sur une caméra réelle."
	}
	return ""
}

// ConnLabel is the French name of a connection state.
func ConnLabel(s core.ConnState) string {
	switch s {
	case core.StateConnected:
		return "Connecté"
	case core.StateConnecting:
		return "Connexion"
	case core.StateError:
		return "Erreur"
	}
	return "Hors ligne"
}

// Decimals is how many decimals to show for a control step.
func Decimals(step float64) int {
	if step <= 0 || math.IsNaN(step) {
		return 2
	}
	for d := 0; d < 4; d++ {
		x := step * math.Pow10(d)
		if math.Abs(x-math.Round(x)) < 1e-9 {
			if d == 0 {
				return 0
			}
			if d == 1 {
				return 1
			}
			return d
		}
	}
	return 3
}

// FormatNumber formats v with a decimal comma.
func FormatNumber(v float64, decimals int) string {
	s := strconv.FormatFloat(v, 'f', decimals, 64)
	if f, err := strconv.ParseFloat(s, 64); err == nil && f == 0 {
		s = strconv.FormatFloat(0, 'f', decimals, 64)
	}
	return strings.Replace(s, ".", ",", 1)
}

// Missing is what an unknown value looks like.
const Missing = "--"

// FormatValue renders a control value for display. A nil or mismatching value
// gives Missing.
func FormatValue(s core.ControlSpec, v *core.Value) string {
	if v == nil || v.Kind != s.Kind {
		return Missing
	}
	switch s.Kind {
	case core.KindRange:
		txt := FormatNumber(v.Number, Decimals(s.Step))
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
			return "Activé"
		}
		return "Désactivé"
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

// NoticeFor maps an error returned by a camera call to a status-bar notice.
func NoticeFor(err error) Notice {
	var ae *app.Error
	if !errors.As(err, &ae) {
		return Notice{"Erreur interne, consultez rcp.log", LevelError}
	}
	switch ae.Code {
	case app.CodeNotImplemented:
		return Notice{"Contrôle pas encore disponible : pilote caméra en développement", LevelInfo}
	case app.CodeInvalidValue:
		return Notice{"Valeur refusée par la caméra", LevelError}
	case app.CodeUnknownControl:
		return Notice{"Contrôle inconnu pour cette caméra", LevelError}
	case app.CodeNotConnected:
		return Notice{"Caméra non connectée", LevelError}
	case app.CodeReadOnly:
		return Notice{"Ce contrôle est en lecture seule", LevelError}
	case app.CodeEmptySlot:
		return Notice{"Aucune caméra sur cet emplacement", LevelError}
	case app.CodeInvalidSlot:
		return Notice{"Emplacement invalide", LevelError}
	}
	return Notice{"Erreur interne, consultez rcp.log", LevelError}
}

// ConfigError turns an AssignSlot or ClearSlot error into an inline French
// message for the configuration form.
func ConfigError(err error) string {
	var ae *app.Error
	if !errors.As(err, &ae) {
		return "Erreur interne, consultez rcp.log."
	}
	switch ae.Code {
	case app.CodeInvalidConfig:
		m := ae.Message
		switch {
		case strings.Contains(m, "host must be"):
			return "L'hôte doit être une adresse IP ou un nom d'hôte valide."
		case strings.Contains(m, "username is required"):
			return "Le nom d'utilisateur est obligatoire pour ce modèle."
		case strings.Contains(m, "out of range"):
			return "Le port doit être compris entre 0 et 65535."
		case strings.Contains(m, "name must be"):
			return "Le nom doit comporter 64 caractères imprimables au maximum."
		case strings.Contains(m, "transport"):
			return "Cette liaison n'est pas disponible pour ce modèle."
		case strings.Contains(m, "credentials must"):
			return "Les identifiants sont trop longs ou contiennent des caractères non valides."
		}
		return "Configuration invalide."
	case app.CodeUnknownModel:
		return "Modèle de caméra inconnu."
	case app.CodeInvalidSlot:
		return "Emplacement invalide."
	}
	return "Erreur interne, consultez rcp.log."
}

// ShortLabel trims a long option label for a compact readout: the part
// before the first " (" ("ND 1.2 (4 stops)" gives "ND 1.2").
func ShortLabel(s string) string {
	if i := strings.Index(s, " ("); i > 0 {
		return s[:i]
	}
	return s
}

// FormatRate formats a byte rate with French units (o/s, ko/s, Mo/s, Go/s),
// binary multiples, one decimal above the byte unit.
func FormatRate(bps float64) string {
	units := []string{"o/s", "ko/s", "Mo/s", "Go/s"}
	i := 0
	for bps >= 1024 && i < len(units)-1 {
		bps /= 1024
		i++
	}
	if i == 0 {
		return FormatNumber(bps, 0) + " " + units[0]
	}
	return FormatNumber(bps, 1) + " " + units[i]
}

// FormatMemory formats used / total memory in gigabytes ("22,3 / 63,3 Go").
func FormatMemory(used, total uint64) string {
	const gib = 1 << 30
	return FormatNumber(float64(used)/gib, 1) + " / " + FormatNumber(float64(total)/gib, 1) + " Go"
}
