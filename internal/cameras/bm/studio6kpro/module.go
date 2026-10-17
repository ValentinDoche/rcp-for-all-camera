// Package studio6kpro is the "bm-studio-6k-pro" camera module: the
// Blackmagic Studio Camera 6K Pro, driven by the shared Blackmagic base
// (package bm) over its Ethernet port. Protocol verified on a real camera.
package studio6kpro

import (
	"log/slog"

	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/bm"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// ID is the module's id.
const ID core.ModelID = "bm-studio-6k-pro"

// Module drives the Studio Camera 6K Pro. The zero value works and logs
// nothing.
type Module struct {
	// Logger receives the commands sent (debug level).
	Logger *slog.Logger
}

var _ core.Module = Module{}

// Info declares the Studio Camera 6K Pro.
func (Module) Info() core.ModelInfo {
	return core.ModelInfo{
		ID: ID, Brand: "Blackmagic Design", Name: "Studio Camera 6K Pro",
		Transports: []core.Transport{core.TransportEthernet}, DefaultPort: bm.DefaultPort,
		Auth: core.AuthNone, Support: core.SupportExperimental, InterchangeableLens: true,
		Notes:   "API REST Blackmagic Camera Control et WebSocket d'événements sur le port Ethernet ; protocole testé sur une vraie caméra, plages de gain, ISO et obturateur à confirmer.",
		Aliases: []core.ModelID{"blackmagic.studio6kpro"},
	}
}

// New builds the camera on the shared base; it does no I/O. The camera
// behaves exactly like the base: no hook is needed for this model.
func (m Module) New(cfg core.CameraConfig) (core.Camera, error) {
	return bm.New(cfg, bm.Model{Specs: Specs(cfg.PowerZoom)}, bm.Options{Logger: m.Logger}), nil
}

// Specs are the controls of the Studio Camera 6K Pro, from the REST API
// the camera serves itself (/control/documentation.html, firmware read on
// 2026-10-11). Its API has no tally, color bars or detail endpoint, so
// those controls are not declared. Zoom needs a power zoom lens.
func Specs(powerZoom bool) []core.ControlSpec {
	var s []core.ControlSpec
	for _, l := range bm.LensSpecs() {
		if l.ID != core.Zoom || powerZoom {
			s = append(s, l)
		}
	}
	s = append(s, bm.GainSpec(), bm.ISOSpec())
	s = append(s, bm.ShutterSpecs("1/24", "1/25", "1/30", "1/48", "1/50", "1/60", "1/100", "1/120", "1/125",
		"1/200", "1/250", "1/500", "1/1000", "1/2000")...)
	s = append(s, bm.NDSpec())
	s = append(s, bm.WhiteSpecs()...)
	s = append(s, bm.ColorCorrectionSpecs()...)
	return append(s, bm.RecordSpec())
}
