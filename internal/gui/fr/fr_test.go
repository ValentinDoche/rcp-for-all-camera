package fr

import (
	"errors"
	"fmt"
	"testing"

	"github.com/valentindoche/rcp-for-all-camera/internal/app"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

func rangeSpec(id core.ControlID, min, max, step float64, unit string) core.ControlSpec {
	s := core.Std(id)
	s.Kind, s.Min, s.Max, s.Step, s.Unit = core.KindRange, min, max, step, unit
	return s
}

func TestControlLabelStandardAndFallback(t *testing.T) {
	if got := ControlLabel(core.Std(core.ShutterAngle)); got != "Angle d'obturateur" {
		t.Errorf("shutterAngle = %q", got)
	}
	if got := ControlLabel(core.Std(core.AutoWhiteBalance)); got != "Balance des blancs auto" {
		t.Errorf("awb = %q", got)
	}
	custom := core.ControlSpec{ID: "sony.slowShutter", Label: "Slow shutter", Group: core.GroupMisc}
	if got := ControlLabel(custom); got != "Slow shutter" {
		t.Errorf("fallback = %q", got)
	}
}

func TestEveryCatalogControlHasFrenchLabel(t *testing.T) {
	for id := range labels {
		if labels[id] == "" {
			t.Errorf("empty label for %s", id)
		}
	}
	for _, id := range []core.ControlID{core.Iris, core.AutoIris, core.Focus, core.AutoFocus, core.Zoom,
		core.Gain, core.ISO, core.ShutterAngle, core.ShutterSpeed, core.ND, core.MasterBlack,
		core.WhiteBalance, core.Tint, core.AutoWhiteBalance, core.LiftR, core.LiftG, core.LiftB, core.LiftY,
		core.GammaR, core.GammaG, core.GammaB, core.GammaY, core.GainR, core.GainG, core.GainB, core.GainY,
		core.Contrast, core.Saturation, core.Hue, core.Detail, core.ColorBars, core.Pan, core.Tilt,
		core.PresetRecall, core.Record, core.Tally} {
		if _, ok := labels[id]; !ok {
			t.Errorf("no French label for standard control %s", id)
		}
	}
}

func TestGroupTitlesCoverAllGroups(t *testing.T) {
	want := map[core.ControlGroup]string{
		core.GroupLens: "Objectif", core.GroupExposure: "Exposition", core.GroupWhite: "Balance des blancs",
		core.GroupColor: "Correction colorimétrique", core.GroupImage: "Image", core.GroupPTZ: "PTZ",
		core.GroupTransport: "Enregistrement", core.GroupAudio: "Audio", core.GroupOutput: "Sorties vidéo",
		core.GroupSystem: "Système", core.GroupMisc: "Divers", "weird": "Divers",
	}
	for g, w := range want {
		if got := GroupTitle(g); got != w {
			t.Errorf("GroupTitle(%q) = %q, want %q", g, got, w)
		}
	}
	if len(GroupOrder) != 11 {
		t.Errorf("GroupOrder has %d entries", len(GroupOrder))
	}
}

func TestFormatNumberFrench(t *testing.T) {
	cases := []struct {
		v    float64
		dec  int
		want string
	}{{0.5, 2, "0,50"}, {1, 2, "1,00"}, {-1.234, 1, "-1,2"}, {5600, 0, "5600"}, {0, 2, "0,00"}, {-0.001, 2, "0,00"}}
	for _, c := range cases {
		if got := FormatNumber(c.v, c.dec); got != c.want {
			t.Errorf("FormatNumber(%v,%d) = %q, want %q", c.v, c.dec, got, c.want)
		}
	}
}

func TestDecimals(t *testing.T) {
	for step, want := range map[float64]int{0: 2, 1: 0, 50: 0, 0.1: 1, 0.01: 2, 0.001: 3, 0.25: 2, 2: 0} {
		if got := Decimals(step); got != want {
			t.Errorf("Decimals(%v) = %d, want %d", step, got, want)
		}
	}
}

func TestFormatValue(t *testing.T) {
	gain := rangeSpec(core.Gain, -12, 36, 2, "dB")
	wb := rangeSpec(core.WhiteBalance, 2500, 10000, 50, "K")
	ang := rangeSpec(core.ShutterAngle, 11.2, 360, 0.1, "°")
	iris := rangeSpec(core.Iris, 0, 1, 0.01, "")
	nd := core.Std(core.ND)
	nd.Kind = core.KindEnum
	nd.Options = []core.Option{{Value: "0", Label: "Clair"}, {Value: "2", Label: "ND 0.6"}}
	tog := core.Std(core.ColorBars)
	tog.Kind = core.KindToggle
	n := func(v float64) *core.Value { x := core.Number(v); return &x }
	ch := func(v string) *core.Value { x := core.Choice(v); return &x }
	b := func(v bool) *core.Value { x := core.Bool(v); return &x }
	cases := []struct {
		name string
		spec core.ControlSpec
		v    *core.Value
		want string
	}{
		{"nil", gain, nil, "--"},
		{"gain+", gain, n(12), "+12 dB"},
		{"gain0", gain, n(0), "0 dB"},
		{"gain-", gain, n(-6), "-6 dB"},
		{"wb", wb, n(5600), "5600 K"},
		{"angle", ang, n(172.8), "172,8°"},
		{"iris", iris, n(0.5), "0,50"},
		{"enum label", nd, ch("2"), "ND 0.6"},
		{"enum unknown", nd, ch("9"), "9"},
		{"toggle on", tog, b(true), "Activé"},
		{"toggle off", tog, b(false), "Désactivé"},
		{"wrong kind", gain, ch("x"), "--"},
	}
	for _, c := range cases {
		if got := FormatValue(c.spec, c.v); got != c.want {
			t.Errorf("%s: FormatValue = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestParseNumber(t *testing.T) {
	cases := []struct {
		in   string
		want float64
		ok   bool
	}{{"1,5", 1.5, true}, {"1.5", 1.5, true}, {" 12 dB ", 12, true}, {"5600K", 5600, true}, {"-0,25", -0.25, true},
		{"+3", 3, true}, {"172,8°", 172.8, true}, {"", 0, false}, {"abc", 0, false}, {"--", 0, false}, {"NaN", 0, false}, {"Inf", 0, false}}
	for _, c := range cases {
		got, ok := ParseNumber(c.in)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("ParseNumber(%q) = %v,%v want %v,%v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestLabelsForSelects(t *testing.T) {
	if TransportLabel(core.TransportUSBEthernet) != "Adaptateur USB-C Ethernet" ||
		TransportLabel(core.TransportEthernet) != "Ethernet" || TransportLabel(core.TransportWiFi) != "Wi-Fi" {
		t.Error("transport labels")
	}
	if SupportLabel(core.SupportPlanned) != "Prévu" || SupportLabel(core.SupportExperimental) != "Expérimental" ||
		SupportLabel(core.SupportSupported) != "Pris en charge" {
		t.Error("support labels")
	}
	if ConnLabel(core.StateConnected) != "Connecté" || ConnLabel(core.StateConnecting) != "Connexion" ||
		ConnLabel(core.StateDisconnected) != "Hors ligne" || ConnLabel(core.StateError) != "Erreur" {
		t.Error("connection labels")
	}
}

func TestErrorNotice(t *testing.T) {
	mk := func(code, msg string) error { return &app.Error{Code: code, Message: msg} }
	cases := []struct {
		err   error
		level Level
		want  string
	}{
		{mk(app.CodeNotImplemented, "x"), LevelInfo, "Contrôle pas encore disponible : pilote caméra en développement"},
		{mk(app.CodeInvalidValue, "x"), LevelError, "Valeur refusée par la caméra"},
		{mk(app.CodeUnknownControl, "x"), LevelError, "Contrôle inconnu pour cette caméra"},
		{mk(app.CodeNotConnected, "x"), LevelError, "Caméra non connectée"},
		{mk(app.CodeReadOnly, "x"), LevelError, "Ce contrôle est en lecture seule"},
		{mk(app.CodeEmptySlot, "x"), LevelError, "Aucune caméra sur cet emplacement"},
		{mk(app.CodeInternal, "x"), LevelError, "Erreur interne, consultez rcp.log"},
		{errors.New("boom"), LevelError, "Erreur interne, consultez rcp.log"},
		{fmt.Errorf("wrap: %w", mk(app.CodeNotImplemented, "x")), LevelInfo, "Contrôle pas encore disponible : pilote caméra en développement"},
	}
	for _, c := range cases {
		n := NoticeFor(c.err)
		if n.Level != c.level || n.Text != c.want {
			t.Errorf("NoticeFor(%v) = %+v, want %v %q", c.err, n, c.level, c.want)
		}
	}
}

func TestConfigErrorFrench(t *testing.T) {
	cases := []struct{ code, msg, want string }{
		{app.CodeInvalidConfig, "acme.cam: host must be an IP address or a host name: invalid camera config", "L'hôte doit être une adresse IP ou un nom d'hôte valide."},
		{app.CodeInvalidConfig, "x: username is required: invalid camera config", "Le nom d'utilisateur est obligatoire pour ce modèle."},
		{app.CodeInvalidConfig, "x: port 70000 out of range: invalid camera config", "Le port doit être compris entre 0 et 65535."},
		{app.CodeInvalidConfig, "x: name must be at most 64 printable characters: invalid camera config", "Le nom doit comporter 64 caractères imprimables au maximum."},
		{app.CodeInvalidConfig, "x: transport \"wifi\" not available on this model: invalid camera config", "Cette liaison n'est pas disponible pour ce modèle."},
		{app.CodeInvalidConfig, "x: credentials must be at most 128 printable characters: invalid camera config", "Les identifiants sont trop longs ou contiennent des caractères non valides."},
		{app.CodeInvalidConfig, "whatever", "Configuration invalide."},
		{app.CodeUnknownModel, "x", "Modèle de caméra inconnu."},
		{app.CodeInvalidSlot, "x", "Emplacement invalide."},
		{app.CodeInternal, "x", "Erreur interne, consultez rcp.log."},
	}
	for _, c := range cases {
		if got := ConfigError(&app.Error{Code: c.code, Message: c.msg}); got != c.want {
			t.Errorf("ConfigError(%s,%q) = %q, want %q", c.code, c.msg, got, c.want)
		}
	}
}

func TestNoEmDashesInStrings(t *testing.T) {
	var all []string
	for _, l := range labels {
		all = append(all, l)
	}
	for _, s := range all {
		for _, r := range s {
			if r == '—' || r == '–' {
				t.Errorf("dash in %q", s)
			}
		}
	}
}

func TestFormatRate(t *testing.T) {
	cases := map[float64]string{
		0:         "0 o/s",
		512:       "512 o/s",
		5329.3:    "5,2 ko/s",
		1_572_864: "1,5 Mo/s",
		3.2e9:     "3,0 Go/s",
	}
	for in, want := range cases {
		if got := FormatRate(in); got != want {
			t.Errorf("FormatRate(%v) = %q, want %q", in, got, want)
		}
	}
}

func TestFormatMemory(t *testing.T) {
	if got := FormatMemory(23915626496, 67939266560); got != "22,3 / 63,3 Go" {
		t.Fatalf("FormatMemory = %q", got)
	}
}
