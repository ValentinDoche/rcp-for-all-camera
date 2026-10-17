// Command sony-generic declares a generic Sony camera for RCP For All Camera.
// Its control protocol is not written yet: the camera page shows every
// control below, and connecting or writing answers "not implemented".
//
// To write the protocol, replace rcpmod.Planned with your own Module
// (see modules/bm-studio-6k-pro) and keep the same id.
//
// Build: go run ./cmd/buildmodules sony-generic
package main

import "github.com/valentindoche/rcp-for-all-camera/sdk/rcpmod"

func main() {}

func init() { rcpmod.Register(module) }

var module = rcpmod.Planned{Model: info, Specs: specs}

var info = rcpmod.Info{
	ID: "sony-generic", Brand: "Sony", Name: "Other Sony model",
	Transports:  []string{rcpmod.TransportEthernet},
	DefaultPort: 80, Auth: rcpmod.AuthRequired, Support: rcpmod.SupportPlanned,
	Notes: "Sony web remote (username and password). The exact model and command set are still to check.",
}

// specs are the camera's controls. Values marked TODO(verify) in the
// original declarations are assumptions to check on the camera.
func specs(cfg rcpmod.Config) []rcpmod.ControlSpec {
	s := []rcpmod.ControlSpec{
		{ID: rcpmod.Iris, Kind: rcpmod.KindRange, Unit: "f", Min: 1.4, Max: 22, Step: 0.1},
		{ID: rcpmod.AutoIris, Kind: rcpmod.KindAction},
		{ID: rcpmod.Focus, Kind: rcpmod.KindRange, Min: 0, Max: 1, Step: 0.001},
		{ID: rcpmod.AutoFocus, Kind: rcpmod.KindAction},
		{ID: rcpmod.ShutterSpeed, Kind: rcpmod.KindEnum, Options: []rcpmod.Option{{Value: "1/24", Label: "1/24"}, {Value: "1/25", Label: "1/25"}, {Value: "1/30", Label: "1/30"}, {Value: "1/48", Label: "1/48"}, {Value: "1/50", Label: "1/50"}, {Value: "1/60", Label: "1/60"}, {Value: "1/100", Label: "1/100"}, {Value: "1/120", Label: "1/120"}, {Value: "1/125", Label: "1/125"}, {Value: "1/250", Label: "1/250"}, {Value: "1/500", Label: "1/500"}, {Value: "1/1000", Label: "1/1000"}, {Value: "1/2000", Label: "1/2000"}}},
		{ID: rcpmod.WhiteBalance, Kind: rcpmod.KindRange, Unit: "K", Min: 2000, Max: 15000, Step: 100},
		{ID: rcpmod.Tint, Kind: rcpmod.KindRange, Min: -7, Max: 7, Step: 1},
		{ID: rcpmod.AutoWhiteBalance, Kind: rcpmod.KindAction},
		{ID: rcpmod.Record, Kind: rcpmod.KindToggle},
		{ID: rcpmod.Tally, Kind: rcpmod.KindToggle, ReadOnly: true},
	}
	return s
}
