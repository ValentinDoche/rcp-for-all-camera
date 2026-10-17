// Command bm-pocket-6k-pro declares the Blackmagic Design Pocket Cinema Camera 6K Pro for RCP For All Camera.
// Its control protocol is not written yet: the camera page shows every
// control below, and connecting or writing answers "not implemented".
//
// To write the protocol, replace rcpmod.Planned with your own Module
// (see modules/bm-studio-6k-pro) and keep the same id.
//
// Build: go run ./cmd/buildmodules bm-pocket-6k-pro
package main

import "github.com/valentindoche/rcp-for-all-camera/sdk/rcpmod"

func main() {}

func init() { rcpmod.Register(module) }

var module = rcpmod.Planned{Model: info, Specs: specs}

var info = rcpmod.Info{
	ID: "bm-pocket-6k-pro", Brand: "Blackmagic Design", Name: "Pocket Cinema Camera 6K Pro",
	Transports:  []string{rcpmod.TransportUSBEthernet},
	DefaultPort: 80, Auth: rcpmod.AuthNone, Support: rcpmod.SupportPlanned,
	InterchangeableLens: true,
	Notes:               "API REST Blackmagic Camera Control via adaptateur USB-C Ethernet ; le Bluetooth n'est pas encore pris en charge et la version de firmware minimale reste à vérifier.",
}

// specs are the camera's controls. Values marked TODO(verify) in the
// original declarations are assumptions to check on the camera.
func specs(cfg rcpmod.Config) []rcpmod.ControlSpec {
	s := []rcpmod.ControlSpec{
		{ID: rcpmod.Iris, Kind: rcpmod.KindRange, Unit: "f", Min: 1, Max: 22, Step: 0.1},
		{ID: rcpmod.IrisNorm, Kind: rcpmod.KindRange, Min: 0, Max: 1, Step: 0.001, Hidden: true},
		{ID: rcpmod.AutoIris, Kind: rcpmod.KindAction},
		{ID: rcpmod.Focus, Kind: rcpmod.KindRange, Min: 0, Max: 1, Step: 0.001},
		{ID: rcpmod.AutoFocus, Kind: rcpmod.KindAction},
		{ID: rcpmod.Zoom, Kind: rcpmod.KindRange, Min: 0, Max: 1, Step: 0.001},
		{ID: rcpmod.ISO, Kind: rcpmod.KindEnum, Options: []rcpmod.Option{{Value: "100", Label: "100"}, {Value: "125", Label: "125"}, {Value: "160", Label: "160"}, {Value: "200", Label: "200"}, {Value: "250", Label: "250"}, {Value: "320", Label: "320"}, {Value: "400", Label: "400"}, {Value: "500", Label: "500"}, {Value: "640", Label: "640"}, {Value: "800", Label: "800"}, {Value: "1000", Label: "1000"}, {Value: "1250", Label: "1250"}, {Value: "1600", Label: "1600"}, {Value: "2000", Label: "2000"}, {Value: "2500", Label: "2500"}, {Value: "3200", Label: "3200"}, {Value: "4000", Label: "4000"}, {Value: "5000", Label: "5000"}, {Value: "6400", Label: "6400"}, {Value: "8000", Label: "8000"}, {Value: "10000", Label: "10000"}, {Value: "12800", Label: "12800"}, {Value: "16000", Label: "16000"}, {Value: "20000", Label: "20000"}, {Value: "25600", Label: "25600"}}},
		{ID: rcpmod.ShutterAngle, Kind: rcpmod.KindRange, Unit: "°", Min: 11.2, Max: 360, Step: 0.1},
		{ID: rcpmod.ShutterSpeed, Kind: rcpmod.KindEnum, Options: []rcpmod.Option{{Value: "1/24", Label: "1/24"}, {Value: "1/25", Label: "1/25"}, {Value: "1/30", Label: "1/30"}, {Value: "1/48", Label: "1/48"}, {Value: "1/50", Label: "1/50"}, {Value: "1/60", Label: "1/60"}, {Value: "1/100", Label: "1/100"}, {Value: "1/120", Label: "1/120"}, {Value: "1/250", Label: "1/250"}, {Value: "1/500", Label: "1/500"}, {Value: "1/1000", Label: "1/1000"}, {Value: "1/2000", Label: "1/2000"}}},
		{ID: rcpmod.ND, Kind: rcpmod.KindEnum, Options: []rcpmod.Option{{Value: "0", Label: "Clear"}, {Value: "2", Label: "ND 0.6 (2 stops)"}, {Value: "4", Label: "ND 1.2 (4 stops)"}, {Value: "6", Label: "ND 1.8 (6 stops)"}}},
		{ID: rcpmod.WhiteBalance, Kind: rcpmod.KindRange, Unit: "K", Min: 2500, Max: 10000, Step: 50},
		{ID: rcpmod.Tint, Kind: rcpmod.KindRange, Min: -50, Max: 50, Step: 1},
		{ID: rcpmod.AutoWhiteBalance, Kind: rcpmod.KindAction},
		{ID: rcpmod.LiftR, Kind: rcpmod.KindRange, Min: -2, Max: 2, Step: 0.01},
		{ID: rcpmod.LiftG, Kind: rcpmod.KindRange, Min: -2, Max: 2, Step: 0.01},
		{ID: rcpmod.LiftB, Kind: rcpmod.KindRange, Min: -2, Max: 2, Step: 0.01},
		{ID: rcpmod.LiftY, Kind: rcpmod.KindRange, Min: -2, Max: 2, Step: 0.01},
		{ID: rcpmod.GammaR, Kind: rcpmod.KindRange, Min: -4, Max: 4, Step: 0.01},
		{ID: rcpmod.GammaG, Kind: rcpmod.KindRange, Min: -4, Max: 4, Step: 0.01},
		{ID: rcpmod.GammaB, Kind: rcpmod.KindRange, Min: -4, Max: 4, Step: 0.01},
		{ID: rcpmod.GammaY, Kind: rcpmod.KindRange, Min: -4, Max: 4, Step: 0.01},
		{ID: rcpmod.GainR, Kind: rcpmod.KindRange, Min: 0, Max: 16, Step: 0.01},
		{ID: rcpmod.GainG, Kind: rcpmod.KindRange, Min: 0, Max: 16, Step: 0.01},
		{ID: rcpmod.GainB, Kind: rcpmod.KindRange, Min: 0, Max: 16, Step: 0.01},
		{ID: rcpmod.GainY, Kind: rcpmod.KindRange, Min: 0, Max: 16, Step: 0.01},
		{ID: rcpmod.Contrast, Kind: rcpmod.KindRange, Min: 0, Max: 2, Step: 0.01},
		{ID: rcpmod.Saturation, Kind: rcpmod.KindRange, Min: 0, Max: 2, Step: 0.01},
		{ID: rcpmod.Hue, Kind: rcpmod.KindRange, Min: -1, Max: 1, Step: 0.01},
		{ID: rcpmod.Detail, Kind: rcpmod.KindEnum, Options: []rcpmod.Option{{Value: "off", Label: "Off"}, {Value: "low", Label: "Low"}, {Value: "medium", Label: "Medium"}, {Value: "high", Label: "High"}}},
		{ID: rcpmod.Record, Kind: rcpmod.KindToggle},
		{ID: rcpmod.Tally, Kind: rcpmod.KindToggle, ReadOnly: true},
	}
	if !cfg.PowerZoom { // the zoom needs a power zoom lens
		return rcpmod.NewSpecs(s).Without(rcpmod.Zoom).All()
	}
	return s
}
