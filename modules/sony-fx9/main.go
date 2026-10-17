// Command sony-fx9 declares the Sony FX9 for RCP For All Camera.
// Its control protocol is not written yet: the camera page shows every
// control below, and connecting or writing answers "not implemented".
//
// To write the protocol, replace rcpmod.Planned with your own Module
// (see modules/bm-studio-6k-pro) and keep the same id.
//
// Build: go run ./cmd/buildmodules sony-fx9
package main

import "github.com/valentindoche/rcp-for-all-camera/sdk/rcpmod"

func main() {}

func init() { rcpmod.Register(module) }

var module = rcpmod.Planned{Model: info, Specs: specs}

var info = rcpmod.Info{
	ID: "sony-fx9", Brand: "Sony", Name: "FX9",
	Transports:  []string{rcpmod.TransportEthernet},
	DefaultPort: 80, Auth: rcpmod.AuthRequired, Support: rcpmod.SupportPlanned,
	Notes: "Sony web remote over the Ethernet port. The available commands are still to check.",
}

// specs are the camera's controls. Values marked TODO(verify) in the
// original declarations are assumptions to check on the camera.
func specs(cfg rcpmod.Config) []rcpmod.ControlSpec {
	s := []rcpmod.ControlSpec{
		{ID: rcpmod.Iris, Kind: rcpmod.KindRange, Unit: "f", Min: 1.4, Max: 22, Step: 0.1},
		{ID: rcpmod.AutoIris, Kind: rcpmod.KindAction},
		{ID: rcpmod.Focus, Kind: rcpmod.KindRange, Min: 0, Max: 1, Step: 0.001},
		{ID: rcpmod.AutoFocus, Kind: rcpmod.KindAction},
		{ID: rcpmod.Zoom, Kind: rcpmod.KindRange, Min: 0, Max: 1, Step: 0.001},
		{ID: rcpmod.ISO, Kind: rcpmod.KindEnum, Options: []rcpmod.Option{{Value: "100", Label: "100"}, {Value: "125", Label: "125"}, {Value: "160", Label: "160"}, {Value: "200", Label: "200"}, {Value: "250", Label: "250"}, {Value: "320", Label: "320"}, {Value: "400", Label: "400"}, {Value: "500", Label: "500"}, {Value: "640", Label: "640"}, {Value: "800", Label: "800"}, {Value: "1000", Label: "1000"}, {Value: "1250", Label: "1250"}, {Value: "1600", Label: "1600"}, {Value: "2000", Label: "2000"}, {Value: "2500", Label: "2500"}, {Value: "3200", Label: "3200"}, {Value: "4000", Label: "4000"}, {Value: "5000", Label: "5000"}, {Value: "6400", Label: "6400"}, {Value: "8000", Label: "8000"}, {Value: "10000", Label: "10000"}, {Value: "12800", Label: "12800"}, {Value: "16000", Label: "16000"}, {Value: "20000", Label: "20000"}, {Value: "25600", Label: "25600"}, {Value: "32000", Label: "32000"}, {Value: "40000", Label: "40000"}, {Value: "51200", Label: "51200"}}},
		{ID: rcpmod.Gain, Kind: rcpmod.KindRange, Unit: "dB", Min: -3, Max: 33, Step: 1},
		{ID: rcpmod.ShutterSpeed, Kind: rcpmod.KindEnum, Options: []rcpmod.Option{{Value: "1/24", Label: "1/24"}, {Value: "1/25", Label: "1/25"}, {Value: "1/30", Label: "1/30"}, {Value: "1/48", Label: "1/48"}, {Value: "1/50", Label: "1/50"}, {Value: "1/60", Label: "1/60"}, {Value: "1/100", Label: "1/100"}, {Value: "1/120", Label: "1/120"}, {Value: "1/125", Label: "1/125"}, {Value: "1/250", Label: "1/250"}, {Value: "1/500", Label: "1/500"}, {Value: "1/1000", Label: "1/1000"}, {Value: "1/2000", Label: "1/2000"}}},
		{ID: rcpmod.ShutterAngle, Kind: rcpmod.KindRange, Unit: "°", Min: 11.25, Max: 360, Step: 0.25},
		{ID: rcpmod.ND, Kind: rcpmod.KindEnum, Options: []rcpmod.Option{{Value: "clear", Label: "Clear"}, {Value: "1/4", Label: "1/4"}, {Value: "1/8", Label: "1/8"}, {Value: "1/16", Label: "1/16"}, {Value: "1/32", Label: "1/32"}, {Value: "1/64", Label: "1/64"}, {Value: "1/128", Label: "1/128"}}},
		{ID: rcpmod.MasterBlack, Kind: rcpmod.KindRange, Min: -15, Max: 15, Step: 1},
		{ID: rcpmod.WhiteBalance, Kind: rcpmod.KindRange, Unit: "K", Min: 2000, Max: 15000, Step: 100},
		{ID: rcpmod.Tint, Kind: rcpmod.KindRange, Min: -7, Max: 7, Step: 1},
		{ID: rcpmod.AutoWhiteBalance, Kind: rcpmod.KindAction},
		{ID: rcpmod.ColorBars, Kind: rcpmod.KindToggle},
		{ID: rcpmod.Record, Kind: rcpmod.KindToggle},
		{ID: rcpmod.Tally, Kind: rcpmod.KindToggle, ReadOnly: true},
	}
	return s
}
