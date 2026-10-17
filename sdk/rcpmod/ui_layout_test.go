package rcpmod_test

import (
	"testing"

	"github.com/valentindoche/rcp-for-all-camera/sdk/rcpmod"
)

var pageControls = []rcpmod.ControlSpec{
	rcpmod.Range(rcpmod.Gain, -12, 36, 2, "dB"),
	rcpmod.Labeled(rcpmod.Toggle("acme.lamp"), "Lamp", rcpmod.GroupSystem),
	rcpmod.Labeled(rcpmod.ReadOnly(rcpmod.ControlSpec{ID: "acme.fw", Kind: rcpmod.KindText}), "Firmware", rcpmod.GroupSystem),
}

func TestStandardPageIsValid(t *testing.T) {
	if err := rcpmod.StandardPage().Check(pageControls); err != nil {
		t.Error(err)
	}
}

func TestPageCheck(t *testing.T) {
	for name, tc := range map[string]struct {
		page rcpmod.Page
		ok   bool
	}{
		"custom block": {rcpmod.StandardPage().AddRow(rcpmod.Controls("Acme", 2,
			rcpmod.With("acme.lamp", rcpmod.WidgetSwitch), rcpmod.With("acme.fw", rcpmod.WidgetReadout))), true},
		"nested grid":          {rcpmod.Page{}.AddRow(rcpmod.Grid(2, rcpmod.Card(rcpmod.BlockIris), rcpmod.Wheel(rcpmod.WheelLift))), true},
		"undeclared control":   {rcpmod.Page{}.AddRow(rcpmod.Controls("X", 1, rcpmod.Show("acme.none"))), false},
		"widget of wrong kind": {rcpmod.Page{}.AddRow(rcpmod.Controls("X", 1, rcpmod.With("gain", rcpmod.WidgetSwitch))), false},
		"unknown block":        {rcpmod.Page{}.AddRow(rcpmod.Block{Type: "carousel"}), false},
		"bad wheel":            {rcpmod.Page{}.AddRow(rcpmod.Wheel("offset")), false},
		"empty row":            {rcpmod.Page{Rows: []rcpmod.Row{{}}}, false},
		"grid too deep": {rcpmod.Page{}.AddRow(rcpmod.Grid(1, rcpmod.Grid(1, rcpmod.Grid(1, rcpmod.Grid(1,
			rcpmod.Card(rcpmod.BlockND)))))), false},
	} {
		t.Run(name, func(t *testing.T) {
			if err := tc.page.Check(pageControls); (err == nil) != tc.ok {
				t.Errorf("Check = %v, want ok %t", err, tc.ok)
			}
		})
	}
}
