// Command panasonic-aw-ue100 declares the Panasonic AW-UE100 for RCP For All Camera.
// Its control protocol is not written yet: the camera page shows every
// control below, and connecting or writing answers "not implemented".
//
// To write the protocol, replace rcpmod.Planned with your own Module
// (see modules/bm-studio-6k-pro) and keep the same id.
//
// Build: go run ./cmd/buildmodules panasonic-aw-ue100
package main

import "github.com/valentindoche/rcp-for-all-camera/sdk/rcpmod"

func main() {}

func init() { rcpmod.Register(module) }

var module = rcpmod.Planned{Model: info, Specs: specs, Page: page}

var info = rcpmod.Info{
	ID: "panasonic-aw-ue100", Brand: "Panasonic", Name: "AW-UE100",
	Transports:  []string{rcpmod.TransportEthernet},
	DefaultPort: 80, Auth: rcpmod.AuthOptional, Support: rcpmod.SupportPlanned,
	Notes: "Panasonic AW HTTP/CGI commands. Whether it has an ND filter, and the ranges, are still to check.",
}

// specs are the camera's controls. Values marked TODO(verify) in the
// original declarations are assumptions to check on the camera.
func specs(cfg rcpmod.Config) []rcpmod.ControlSpec {
	s := []rcpmod.ControlSpec{
		{ID: rcpmod.Iris, Kind: rcpmod.KindRange, Min: 0, Max: 1, Step: 0.01},
		{ID: rcpmod.AutoIris, Kind: rcpmod.KindAction},
		{ID: rcpmod.Focus, Kind: rcpmod.KindRange, Min: 0, Max: 1, Step: 0.001},
		{ID: rcpmod.AutoFocus, Kind: rcpmod.KindAction},
		{ID: rcpmod.Gain, Kind: rcpmod.KindRange, Unit: "dB", Min: 0, Max: 42, Step: 1},
		{ID: rcpmod.ShutterSpeed, Kind: rcpmod.KindEnum, Options: []rcpmod.Option{{Value: "1/24", Label: "1/24"}, {Value: "1/25", Label: "1/25"}, {Value: "1/30", Label: "1/30"}, {Value: "1/50", Label: "1/50"}, {Value: "1/60", Label: "1/60"}, {Value: "1/100", Label: "1/100"}, {Value: "1/120", Label: "1/120"}, {Value: "1/250", Label: "1/250"}, {Value: "1/500", Label: "1/500"}, {Value: "1/1000", Label: "1/1000"}, {Value: "1/2000", Label: "1/2000"}, {Value: "1/10000", Label: "1/10000"}}},
		{ID: rcpmod.WhiteBalance, Kind: rcpmod.KindRange, Unit: "K", Min: 2000, Max: 15000, Step: 100},
		{ID: rcpmod.AutoWhiteBalance, Kind: rcpmod.KindAction},
		{ID: rcpmod.MasterBlack, Kind: rcpmod.KindRange, Min: -150, Max: 150, Step: 1},
		{ID: rcpmod.Detail, Kind: rcpmod.KindRange, Min: -31, Max: 31, Step: 1},
		{ID: rcpmod.ColorBars, Kind: rcpmod.KindToggle},
		{ID: rcpmod.Pan, Kind: rcpmod.KindRange, Min: -1, Max: 1, Step: 0.01},
		{ID: rcpmod.Tilt, Kind: rcpmod.KindRange, Min: -1, Max: 1, Step: 0.01},
		{ID: rcpmod.Zoom, Kind: rcpmod.KindRange, Min: 0, Max: 1, Step: 0.001},
		{ID: rcpmod.PresetRecall, Kind: rcpmod.KindEnum, Options: []rcpmod.Option{{Value: "1", Label: "1"}, {Value: "2", Label: "2"}, {Value: "3", Label: "3"}, {Value: "4", Label: "4"}, {Value: "5", Label: "5"}, {Value: "6", Label: "6"}, {Value: "7", Label: "7"}, {Value: "8", Label: "8"}, {Value: "9", Label: "9"}, {Value: "10", Label: "10"}, {Value: "11", Label: "11"}, {Value: "12", Label: "12"}, {Value: "13", Label: "13"}, {Value: "14", Label: "14"}, {Value: "15", Label: "15"}, {Value: "16", Label: "16"}, {Value: "17", Label: "17"}, {Value: "18", Label: "18"}, {Value: "19", Label: "19"}, {Value: "20", Label: "20"}, {Value: "21", Label: "21"}, {Value: "22", Label: "22"}, {Value: "23", Label: "23"}, {Value: "24", Label: "24"}, {Value: "25", Label: "25"}, {Value: "26", Label: "26"}, {Value: "27", Label: "27"}, {Value: "28", Label: "28"}, {Value: "29", Label: "29"}, {Value: "30", Label: "30"}, {Value: "31", Label: "31"}, {Value: "32", Label: "32"}, {Value: "33", Label: "33"}, {Value: "34", Label: "34"}, {Value: "35", Label: "35"}, {Value: "36", Label: "36"}, {Value: "37", Label: "37"}, {Value: "38", Label: "38"}, {Value: "39", Label: "39"}, {Value: "40", Label: "40"}, {Value: "41", Label: "41"}, {Value: "42", Label: "42"}, {Value: "43", Label: "43"}, {Value: "44", Label: "44"}, {Value: "45", Label: "45"}, {Value: "46", Label: "46"}, {Value: "47", Label: "47"}, {Value: "48", Label: "48"}, {Value: "49", Label: "49"}, {Value: "50", Label: "50"}, {Value: "51", Label: "51"}, {Value: "52", Label: "52"}, {Value: "53", Label: "53"}, {Value: "54", Label: "54"}, {Value: "55", Label: "55"}, {Value: "56", Label: "56"}, {Value: "57", Label: "57"}, {Value: "58", Label: "58"}, {Value: "59", Label: "59"}, {Value: "60", Label: "60"}, {Value: "61", Label: "61"}, {Value: "62", Label: "62"}, {Value: "63", Label: "63"}, {Value: "64", Label: "64"}, {Value: "65", Label: "65"}, {Value: "66", Label: "66"}, {Value: "67", Label: "67"}, {Value: "68", Label: "68"}, {Value: "69", Label: "69"}, {Value: "70", Label: "70"}, {Value: "71", Label: "71"}, {Value: "72", Label: "72"}, {Value: "73", Label: "73"}, {Value: "74", Label: "74"}, {Value: "75", Label: "75"}, {Value: "76", Label: "76"}, {Value: "77", Label: "77"}, {Value: "78", Label: "78"}, {Value: "79", Label: "79"}, {Value: "80", Label: "80"}, {Value: "81", Label: "81"}, {Value: "82", Label: "82"}, {Value: "83", Label: "83"}, {Value: "84", Label: "84"}, {Value: "85", Label: "85"}, {Value: "86", Label: "86"}, {Value: "87", Label: "87"}, {Value: "88", Label: "88"}, {Value: "89", Label: "89"}, {Value: "90", Label: "90"}, {Value: "91", Label: "91"}, {Value: "92", Label: "92"}, {Value: "93", Label: "93"}, {Value: "94", Label: "94"}, {Value: "95", Label: "95"}, {Value: "96", Label: "96"}, {Value: "97", Label: "97"}, {Value: "98", Label: "98"}, {Value: "99", Label: "99"}, {Value: "100", Label: "100"}}},
		{ID: rcpmod.Record, Kind: rcpmod.KindToggle},
		{ID: rcpmod.Tally, Kind: rcpmod.KindToggle, ReadOnly: true},
	}
	return s
}
