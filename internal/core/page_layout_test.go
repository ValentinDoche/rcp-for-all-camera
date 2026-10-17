package core_test

import (
	"testing"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

func TestPageCheck(t *testing.T) {
	specs := []core.ControlSpec{
		{ID: core.Gain, Kind: core.KindRange, Max: 36},
		{ID: "acme.fw", Kind: core.KindText, ReadOnly: true},
	}
	ctl := func(items ...core.Item) core.Page {
		return core.Page{Rows: []core.Row{{Blocks: []core.Block{{Type: core.BlockControls, Title: "Acme", Items: items}}}}}
	}
	for name, tc := range map[string]struct {
		page core.Page
		ok   bool
	}{
		"standard page":      {core.StandardPage(), true},
		"readout":            {ctl(core.Item{Control: "acme.fw", Widget: core.WidgetReadout}), true},
		"stepper":            {ctl(core.Item{Control: core.Gain, Widget: core.WidgetStepper}), true},
		"undeclared control": {ctl(core.Item{Control: "acme.none"}), false},
		"wrong widget":       {ctl(core.Item{Control: core.Gain, Widget: core.WidgetSwitch}), false},
		"unknown type":       {core.Page{Rows: []core.Row{{Blocks: []core.Block{{Type: "x"}}}}}, false},
		"empty row":          {core.Page{Rows: []core.Row{{}}}, false},
	} {
		t.Run(name, func(t *testing.T) {
			if err := tc.page.Check(specs); (err == nil) != tc.ok {
				t.Errorf("Check = %v, want ok %t", err, tc.ok)
			}
		})
	}
}
