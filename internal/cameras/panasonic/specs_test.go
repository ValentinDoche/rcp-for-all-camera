package panasonic

import (
	"strings"
	"testing"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

const (
	ue150 core.ModelID = "panasonic.awue150"
	ue160 core.ModelID = "panasonic.awue160"
)

func wantRange(t *testing.T, id core.ModelID, cid core.ControlID, min, max float64) {
	t.Helper()
	s, ok := controls(t, id)[cid]
	if !ok {
		t.Errorf("%s lacks %s", id, cid)
		return
	}
	if s.Kind != core.KindRange || s.Min != min || s.Max != max {
		t.Errorf("%s %s = kind %s [%v,%v], want range [%v,%v]", id, cid, s.Kind, s.Min, s.Max, min, max)
	}
}

func wantOptions(t *testing.T, id core.ModelID, cid core.ControlID, n int) core.ControlSpec {
	t.Helper()
	s, ok := controls(t, id)[cid]
	if !ok {
		t.Errorf("%s lacks %s", id, cid)
		return s
	}
	if s.Kind != core.KindEnum || len(s.Options) != n {
		t.Errorf("%s %s = kind %s with %d options, want enum with %d", id, cid, s.Kind, len(s.Options), n)
	}
	return s
}

func TestFullListSize(t *testing.T) {
	for _, id := range []core.ModelID{ue150, ue160} {
		if n := len(controls(t, id)); n < 180 {
			t.Errorf("%s declares %d controls, want >= 180", id, n)
		}
	}
}

func TestSampleRanges(t *testing.T) {
	wantRange(t, ue160, core.MasterBlack, -800, 800)
	wantRange(t, ue150, core.MasterBlack, -200, 200)
	wantRange(t, ue160, core.LiftR, -800, 800)
	wantRange(t, ue150, core.LiftR, -100, 100)
	wantRange(t, ue160, "panasonic.linearMatrixRG_P", -31, 31)
	wantRange(t, ue150, "panasonic.linearMatrixRG", -63, 63)
	wantRange(t, ue160, core.Iris, 1.4, 25.5)
	wantRange(t, ue150, core.Iris, 1.4, 25.5)
	wantRange(t, ue160, core.Saturation, -100, 40)
	wantRange(t, ue150, core.Saturation, -99, 99)
	wantRange(t, ue160, core.GainR, -1000, 1000)
	wantRange(t, ue150, core.GainR, -200, 200)
	wantRange(t, ue160, core.WhiteBalance, 2000, 15000)
	wantRange(t, ue160, core.Focus, 0, 1)
	wantRange(t, ue160, core.Zoom, 0, 1)
	wantRange(t, ue160, "panasonic.kneePointMaster", 70, 107)
	wantRange(t, ue160, "panasonic.cropZoomRatio", 120, 500)
	wantRange(t, ue150, "panasonic.cropHPositionG", 0, 2560)
}

func TestSampleKinds(t *testing.T) {
	for _, id := range []core.ModelID{ue150, ue160} {
		c := controls(t, id)
		for cid, kind := range map[core.ControlID]core.ControlKind{
			core.AutoFocus:         core.KindAction,
			core.AutoWhiteBalance:  core.KindAction,
			core.ColorBars:         core.KindToggle,
			"panasonic.irisMode":   core.KindEnum,
			"panasonic.presetHome": core.KindAction,
			"panasonic.tallyR":     core.KindToggle,
			"panasonic.osdMenuUp":  core.KindAction,
		} {
			if s, ok := c[cid]; !ok || s.Kind != kind {
				t.Errorf("%s %s = %+v, want kind %s", id, cid, s, kind)
			}
		}
		if _, ok := c[core.AutoIris]; ok {
			t.Errorf("%s: autoIris has no one-touch trigger in the list", id)
		}
	}
}

func TestGainOptions(t *testing.T) {
	s := wantOptions(t, ue160, core.Gain, 20)
	has := map[string]string{}
	for _, o := range s.Options {
		has[o.Value] = o.Label
	}
	if has["-6"] != "-6 dB" || has["0"] != "0 dB" || has["12"] != "12 dB" || has["auto"] != "Auto" {
		t.Errorf("UE160 gain options: %v", has)
	}
	s = wantOptions(t, ue150, core.Gain, 44)
	has = map[string]string{}
	for _, o := range s.Options {
		has[o.Value] = o.Label
	}
	if has["42"] != "42 dB" || has["auto"] != "Auto" || has["-6"] != "" {
		t.Errorf("UE150 gain options: %v", has)
	}
}

func TestOptionCounts(t *testing.T) {
	wantOptions(t, ue160, "panasonic.12GSDIOutputFormat", 18)
	wantOptions(t, ue150, "panasonic.12GSDIOutputFormat", 20)
	wantOptions(t, ue160, "panasonic.hdmiOutputFormat", 20)
	wantOptions(t, ue160, core.PresetRecall, 100)
	wantOptions(t, ue160, "panasonic.presetStore", 100)
	wantOptions(t, ue160, "panasonic.presetDelete", 100)
	wantOptions(t, ue160, core.ShutterSpeed, 8)
	wantOptions(t, ue150, core.ShutterSpeed, 10)
	wantOptions(t, ue160, core.ND, 4)
	wantOptions(t, ue160, "panasonic.sceneFile", 9)
	wantOptions(t, ue150, "panasonic.sceneFile", 4)
}

func TestExcluded(t *testing.T) {
	for _, id := range []core.ModelID{ue150, ue160} {
		for cid := range controls(t, id) {
			s := strings.ToLower(string(cid))
			for _, bad := range []string{"device_ip", "device_test", "dctrace", "dccruise", "presetthumbnail",
				"presetname", "presetexists", "incremental", "cameraname", "firmware", "colortemperaturevalue",
				"panasonic.connection"} {
				if strings.Contains(s, bad) {
					t.Errorf("%s declares excluded control %s", id, cid)
				}
			}
		}
	}
}

func TestIDsAndGroups(t *testing.T) {
	std := map[core.ControlID]bool{}
	for _, cid := range []core.ControlID{core.Iris, core.Focus, core.AutoFocus, core.Zoom, core.ND, core.Gain,
		core.ShutterSpeed, core.WhiteBalance, core.AutoWhiteBalance, core.MasterBlack, core.LiftR, core.LiftG,
		core.LiftB, core.GainR, core.GainG, core.GainB, core.Saturation, core.Detail, core.ColorBars,
		core.Pan, core.Tilt, core.PresetRecall} {
		std[cid] = true
	}
	for _, id := range []core.ModelID{ue150, ue160} {
		for cid, s := range controls(t, id) {
			if s.Group == "" || s.Label == "" {
				t.Errorf("%s %s: empty group or label", id, cid)
			}
			if !std[cid] && !strings.HasPrefix(string(cid), "panasonic.") {
				t.Errorf("%s: non-standard id %s lacks the panasonic. prefix", id, cid)
			}
		}
	}
}

func TestModelSpecificParameters(t *testing.T) {
	// UE160 only: green color gain, SDI crop output, Frame/HLG black gamma.
	if _, ok := controls(t, ue150)[core.GainG]; ok {
		t.Error("UE150 has no green color gain")
	}
	if _, ok := controls(t, ue160)[core.GainG]; !ok {
		t.Error("UE160 lacks green color gain")
	}
	if _, ok := controls(t, ue150)["panasonic.skinToneDTLICenter"]; ok {
		t.Error("UE150 has no skin tone DTL I center")
	}
	if _, ok := controls(t, ue160)["panasonic.skinToneDTLICenter"]; !ok {
		t.Error("UE160 lacks skin tone DTL I center")
	}
}

func TestOlderModelsUnchanged(t *testing.T) {
	if n := len(controls(t, "panasonic.generic")); n == 0 {
		t.Error("generic model has no controls")
	}
	if _, ok := controls(t, "panasonic.awue100")[core.Pan]; !ok {
		t.Error("UE100 lacks pan")
	}
}

func TestOnAirAndPowerControlsAreGuarded(t *testing.T) {
	for _, id := range []core.ModelID{"panasonic.awue150", "panasonic.awue160"} {
		specs, err := specsFor(id)
		if err != nil {
			t.Fatal(err)
		}
		byID := map[core.ControlID]core.ControlSpec{}
		for _, s := range specs {
			byID[s.ID] = s
		}
		for _, g := range []core.ControlID{"panasonic.power", "panasonic.frequency", "panasonic.systemFormat",
			"panasonic.sdiOutputFormat", "panasonic.hdmiOutputFormat", "panasonic.osdMenuEnable",
			"panasonic.sceneFile", "panasonic.presetStore", "panasonic.presetDelete", core.ColorBars} {
			if s, ok := byID[g]; ok && !s.Guarded {
				t.Errorf("%s: %s must be guarded", id, g)
			}
		}
		if byID[core.Iris].Guarded || byID["panasonic.kneePointMaster"].Guarded {
			t.Errorf("%s: picture controls must not be guarded", id)
		}
	}
}
