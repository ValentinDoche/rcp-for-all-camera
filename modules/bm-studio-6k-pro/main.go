// Command bm-studio-6k-pro is the camera module of the Blackmagic Studio
// Camera 6K Pro. It speaks the Blackmagic Camera Control REST API over the
// camera's Ethernet port: writes are PUTs under /control/api/v1, state comes
// from the event WebSocket at /control/api/v1/event/websocket.
//
// The camera serves its own API documentation at
// http://<camera>/control/documentation.html; openapi/ keeps the copy this
// module was written from.
//
// Build: go run ./cmd/buildmodules bm-studio-6k-pro
package main

import "github.com/valentindoche/rcp-for-all-camera/sdk/rcpmod"

func main() {}

func init() { rcpmod.Register(module{}) }

type module struct{}

func (module) Info() rcpmod.Info {
	return rcpmod.Info{
		ID: "bm-studio-6k-pro", Brand: "Blackmagic Design", Name: "Studio Camera 6K Pro",
		Transports: []string{rcpmod.TransportEthernet}, DefaultPort: 80,
		Auth: rcpmod.AuthNone, Support: rcpmod.SupportExperimental, InterchangeableLens: true,
		Notes: "API REST Blackmagic Camera Control et WebSocket d'événements sur le port Ethernet ; protocole testé sur une vraie caméra, plages de gain, ISO et obturateur à confirmer.",
	}
}

// New builds the camera; it does no I/O.
func (module) New(cfg rcpmod.Config, host rcpmod.Host) (rcpmod.Camera, error) {
	return newCamera(host, specs(cfg.PowerZoom)), nil
}

// specs are the controls of the Studio Camera 6K Pro, from the REST API the
// camera serves (firmware read on 2026-10-11). Its API has no tally, color
// bars or detail endpoint, so those controls are not declared. Zoom needs a
// power zoom lens.
func specs(powerZoom bool) []rcpmod.ControlSpec {
	// IrisNorm is the iris over the lens' range ("normalised" of
	// /lens/iris), for the panel's lever: the camera maps it to the mounted
	// lens, so the lever never asks for a stop the lens lacks. 0 = open,
	// 1 = closed. TODO(verify) the direction on a live camera.
	irisNorm := rcpmod.Range(rcpmod.IrisNorm, 0, 1, 0.001, "")
	irisNorm.Hidden = true
	s := []rcpmod.ControlSpec{
		// f-stops (apertureStop); the camera snaps to the lens' stops.
		// TODO(verify): f/1.0 to f/22 covers the lenses seen so far.
		rcpmod.Range(rcpmod.Iris, 1, 22, 0.1, "f"),
		irisNorm,
		rcpmod.Action(rcpmod.AutoIris),
		rcpmod.Range(rcpmod.Focus, 0, 1, 0.001, ""),
		rcpmod.Action(rcpmod.AutoFocus),
	}
	if powerZoom {
		s = append(s, rcpmod.Range(rcpmod.Zoom, 0, 1, 0.001, ""))
	}
	s = append(s,
		// TODO(verify): the API accepts any dB value; the usable range is the camera's.
		rcpmod.Range(rcpmod.Gain, -12, 36, 2, "dB"),
		// TODO(verify): the API takes any integer; these are the menu steps.
		rcpmod.Enum(rcpmod.ISO, rcpmod.Opts(
			"100", "125", "160", "200", "250", "320", "400", "500", "640", "800",
			"1000", "1250", "1600", "2000", "2500", "3200", "4000", "5000", "6400", "8000",
			"10000", "12800", "16000", "20000", "25600")),
		// TODO(verify): the minimum angle depends on the frame rate.
		rcpmod.Range(rcpmod.ShutterAngle, 11.2, 360, 0.1, "°"),
		rcpmod.Enum(rcpmod.ShutterSpeed, rcpmod.Opts(
			"1/24", "1/25", "1/30", "1/48", "1/50", "1/60", "1/100", "1/120", "1/125",
			"1/200", "1/250", "1/500", "1/1000", "1/2000")),
		// TODO(verify): positions in stops as exposed by /video/ndFilter.
		rcpmod.Enum(rcpmod.ND, rcpmod.Pairs("0", "Clear", "2", "ND 0.6 (2 stops)", "4", "ND 1.2 (4 stops)", "6", "ND 1.8 (6 stops)")),
		rcpmod.Range(rcpmod.WhiteBalance, 2500, 10000, 50, "K"),
		rcpmod.Range(rcpmod.Tint, -50, 50, 1, ""),
		rcpmod.Action(rcpmod.AutoWhiteBalance),
	)
	for _, id := range []string{rcpmod.LiftR, rcpmod.LiftG, rcpmod.LiftB, rcpmod.LiftY, rcpmod.MasterBlack} {
		s = append(s, rcpmod.Range(id, -2, 2, 0.01, ""))
	}
	for _, id := range []string{rcpmod.GammaR, rcpmod.GammaG, rcpmod.GammaB, rcpmod.GammaY} {
		s = append(s, rcpmod.Range(id, -4, 4, 0.01, ""))
	}
	for _, id := range []string{rcpmod.GainR, rcpmod.GainG, rcpmod.GainB, rcpmod.GainY} {
		s = append(s, rcpmod.Range(id, 0, 16, 0.01, ""))
	}
	for _, o := range []struct{ id, label string }{
		{offsetR, "Offset R"}, {offsetG, "Offset G"}, {offsetB, "Offset B"}, {offsetY, "Offset Y"},
	} {
		s = append(s, rcpmod.Labeled(rcpmod.Range(o.id, -8, 8, 0.01, ""), o.label, rcpmod.GroupColor))
	}
	return append(s,
		rcpmod.Range(rcpmod.Contrast, 0, 2, 0.01, ""),
		rcpmod.Labeled(rcpmod.Range(contrastPivot, 0, 1, 0.01, ""), "Contrast pivot", rcpmod.GroupColor),
		rcpmod.Range(rcpmod.Saturation, 0, 2, 0.01, ""),
		rcpmod.Range(rcpmod.Hue, -1, 1, 0.01, ""),
		rcpmod.Labeled(rcpmod.Range(lumaContribution, 0, 1, 0.01, ""), "Luma mix", rcpmod.GroupColor),
		rcpmod.Toggle(rcpmod.Record),
	)
}
