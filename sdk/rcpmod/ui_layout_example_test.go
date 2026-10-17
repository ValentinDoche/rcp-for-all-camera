package rcpmod_test

import (
	"encoding/json"
	"fmt"

	"github.com/valentindoche/rcp-for-all-camera/sdk/rcpmod"
)

// A module that has nothing special to show returns the standard page.
func ExampleStandardPage() {
	b, _ := json.Marshal(rcpmod.StandardPage())
	fmt.Println(string(b))
	// Output: {"rows":[{"blocks":[{"type":"exposure"}]},{"blocks":[{"type":"color"}]}]}
}

// The standard page with the module's own controls added at the end, each
// with the widget it should use.
func ExamplePage_AddRow() {
	page := rcpmod.StandardPage().AddRow(rcpmod.Controls("Tally", 2,
		rcpmod.With("acme.tallyLamp", rcpmod.WidgetSwitch),
		rcpmod.Show("acme.tallyBrightness"),
	))
	b, _ := json.Marshal(page.Rows[2])
	fmt.Println(string(b))
	// Output: {"blocks":[{"type":"controls","title":"Tally","items":[{"control":"acme.tallyLamp","widget":"switch"},{"control":"acme.tallyBrightness"}],"columns":2}]}
}

// Check reports a page that names a control the camera does not declare,
// before the application ever shows it.
func ExamplePage_Check() {
	controls := []rcpmod.ControlSpec{rcpmod.Toggle("acme.tallyLamp")}
	good := rcpmod.StandardPage().AddRow(rcpmod.Controls("Tally", 1, rcpmod.With("acme.tallyLamp", rcpmod.WidgetSwitch)))
	bad := rcpmod.StandardPage().AddRow(rcpmod.Controls("Tally", 1, rcpmod.Show("acme.missing")))
	fmt.Println(good.Check(controls) == nil)
	fmt.Println(bad.Check(controls) == nil)
	// Output:
	// true
	// false
}

// A custom page: the iris card beside the lift wheel, then the presets and
// every control not placed yet.
func ExampleGrid() {
	page := rcpmod.Page{}.
		AddRow(rcpmod.Grid(2, rcpmod.Card(rcpmod.BlockIris), rcpmod.Wheel(rcpmod.WheelLift))).
		AddRow(rcpmod.Card(rcpmod.BlockPresets)).
		AddRow(rcpmod.Remaining())
	fmt.Println(len(page.Rows), page.Rows[0].Blocks[0].Type, page.Rows[2].Blocks[0].Type)
	// Output: 3 grid remaining
}
