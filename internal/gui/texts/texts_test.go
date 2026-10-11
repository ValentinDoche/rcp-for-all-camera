package texts

import (
	"errors"
	"fmt"
	"testing"

	"github.com/valentindoche/rcp-for-all-camera/internal/app"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
	"github.com/valentindoche/rcp-for-all-camera/internal/i18n"
)

func en() Texts { return New(i18n.Default().Translator("en")) }

func rangeSpec(id core.ControlID, min, max, step float64, unit string) core.ControlSpec {
	s := core.Std(id)
	s.Kind, s.Min, s.Max, s.Step, s.Unit = core.KindRange, min, max, step, unit
	return s
}

func TestControlLabelStandardAndFallback(t *testing.T) {
	fr := Default()
	if got := fr.ControlLabel(core.Std(core.ShutterAngle)); got != "Angle d'obturateur" {
		t.Errorf("shutterAngle = %q", got)
	}
	if got := en().ControlLabel(core.Std(core.ShutterAngle)); got != "Shutter angle" {
		t.Errorf("en shutterAngle = %q", got)
	}
	custom := core.ControlSpec{ID: "sony.slowShutter", Label: "Slow shutter", Group: core.GroupMisc}
	for _, x := range []Texts{fr, en()} {
		if got := x.ControlLabel(custom); got != "Slow shutter" {
			t.Errorf("driver label = %q", got)
		}
	}
	if got := fr.ControlLabel(core.ControlSpec{ID: "x.y"}); got != "x.y" {
		t.Errorf("id fallback = %q", got)
	}
}

func TestEveryStandardControlAndGroupIsTranslated(t *testing.T) {
	ids := []core.ControlID{core.Iris, core.AutoIris, core.Focus, core.AutoFocus, core.Zoom,
		core.Gain, core.ISO, core.ShutterAngle, core.ShutterSpeed, core.ND, core.MasterBlack,
		core.WhiteBalance, core.Tint, core.AutoWhiteBalance, core.LiftR, core.LiftG, core.LiftB, core.LiftY,
		core.GammaR, core.GammaG, core.GammaB, core.GammaY, core.GainR, core.GainG, core.GainB, core.GainY,
		core.Contrast, core.Saturation, core.Hue, core.Detail, core.ColorBars, core.Pan, core.Tilt,
		core.PresetRecall, core.Record, core.Tally}
	for _, l := range i18n.Default().Languages() {
		tr := i18n.Default().Translator(l.Code)
		for _, id := range ids {
			if !tr.Has("control." + string(id)) {
				t.Errorf("%s: no label for standard control %s", l.Code, id)
			}
		}
		for _, g := range GroupOrder {
			if !tr.Has("group." + string(g)) {
				t.Errorf("%s: no title for group %s", l.Code, g)
			}
		}
	}
}

func TestGroupTitles(t *testing.T) {
	fr := Default()
	want := map[core.ControlGroup]string{
		core.GroupLens: "Objectif", core.GroupExposure: "Exposition", core.GroupWhite: "Balance des blancs",
		core.GroupColor: "Correction colorimétrique", core.GroupImage: "Image", core.GroupPTZ: "PTZ",
		core.GroupTransport: "Enregistrement", core.GroupAudio: "Audio", core.GroupOutput: "Sorties vidéo",
		core.GroupSystem: "Système", core.GroupMisc: "Divers", "weird": "Divers",
	}
	for g, w := range want {
		if got := fr.GroupTitle(g); got != w {
			t.Errorf("GroupTitle(%q) = %q, want %q", g, got, w)
		}
	}
	if en().GroupTitle("weird") != "Other" || en().GroupTitle(core.GroupLens) != "Lens" {
		t.Error("en group titles")
	}
	if len(GroupOrder) != 11 {
		t.Errorf("GroupOrder has %d entries", len(GroupOrder))
	}
}

func TestFormatNumber(t *testing.T) {
	cases := []struct {
		v        float64
		dec      int
		fr, want string
	}{{0.5, 2, "0,50", "0.50"}, {-1.234, 1, "-1,2", "-1.2"}, {5600, 0, "5600", "5600"}, {-0.001, 2, "0,00", "0.00"}}
	for _, c := range cases {
		if got := Default().FormatNumber(c.v, c.dec); got != c.fr {
			t.Errorf("fr FormatNumber(%v,%d) = %q, want %q", c.v, c.dec, got, c.fr)
		}
		if got := en().FormatNumber(c.v, c.dec); got != c.want {
			t.Errorf("en FormatNumber(%v,%d) = %q, want %q", c.v, c.dec, got, c.want)
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
		if got := Default().FormatValue(c.spec, c.v); got != c.want {
			t.Errorf("%s: FormatValue = %q, want %q", c.name, got, c.want)
		}
	}
	if got := en().FormatValue(ang, n(172.8)); got != "172.8°" {
		t.Errorf("en angle = %q", got)
	}
	if got := en().FormatValue(tog, b(true)); got != "On" {
		t.Errorf("en toggle = %q", got)
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
	fr := Default()
	if fr.SupportLabel(core.SupportPlanned) != "Prévu" || fr.SupportLabel(core.SupportExperimental) != "Expérimental" ||
		fr.SupportLabel(core.SupportSupported) != "Pris en charge" {
		t.Error("support labels")
	}
	if fr.ConnLabel(core.StateConnected) != "Connecté" || fr.ConnLabel(core.StateConnecting) != "Connexion" ||
		fr.ConnLabel(core.StateDisconnected) != "Hors ligne" || fr.ConnLabel(core.StateError) != "Erreur" {
		t.Error("connection labels")
	}
	if en().SupportLabel(core.SupportSupported) != "Supported" || en().ConnLabel(core.StateDisconnected) != "Offline" {
		t.Error("en labels")
	}
	for _, s := range []core.SupportLevel{core.SupportPlanned, core.SupportExperimental, core.SupportSupported} {
		if fr.SupportHelp(s) == "" || en().SupportHelp(s) == "" {
			t.Errorf("no help for %s", s)
		}
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
		n := Default().NoticeFor(c.err)
		if n.Level != c.level || n.Text != c.want {
			t.Errorf("NoticeFor(%v) = %+v, want %v %q", c.err, n, c.level, c.want)
		}
	}
	if n := en().NoticeFor(mk(app.CodeNotConnected, "")); n.Text != "Camera not connected" {
		t.Errorf("en notice = %q", n.Text)
	}
}

func TestConfigError(t *testing.T) {
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
		if got := Default().ConfigError(&app.Error{Code: c.code, Message: c.msg}); got != c.want {
			t.Errorf("ConfigError(%s,%q) = %q, want %q", c.code, c.msg, got, c.want)
		}
	}
	if got := en().ConfigError(&app.Error{Code: app.CodeInvalidConfig, Message: "x: port 70000 out of range"}); got != "The port must be between 0 and 65535." {
		t.Errorf("en ConfigError = %q", got)
	}
}

func TestSettingsErrors(t *testing.T) {
	ni := &app.Error{Code: app.CodeNotImplemented}
	in := &app.Error{Code: app.CodeInternal}
	for _, x := range []Texts{Default(), en()} {
		if x.AutoStartError(ni) == x.AutoStartError(in) {
			t.Error("auto start errors must differ")
		}
		if x.BindError(&app.Error{Code: app.CodeInvalidConfig}) == x.BindError(in) {
			t.Error("bind errors must differ")
		}
	}
}

func TestShortLabel(t *testing.T) {
	if ShortLabel("ND 1.2 (4 stops)") != "ND 1.2" || ShortLabel("Clair") != "Clair" {
		t.Error("ShortLabel")
	}
}
