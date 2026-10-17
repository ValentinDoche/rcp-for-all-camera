// Package planned declares the Blackmagic camera modules whose protocol is
// not verified yet. Each is a camerakit.Planned module: the GUI shows its
// controls, any I/O answers NOT_IMPLEMENTED. Once a model is checked on
// hardware it gets its own module package on the bm base, with the same id
// (as bm/studio6kpro did).
package planned

import (
	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/bm"
	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/camerakit"
	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/speckit"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// Modules returns the planned Blackmagic modules, in display order.
func Modules() []core.Module {
	return []core.Module{
		pocket("bm-pocket-4k", "blackmagic.pocket4k", "Pocket Cinema Camera 4K", false),
		pocket("bm-pocket-6k-g2", "blackmagic.pocket6kg2", "Pocket Cinema Camera 6K G2", false),
		pocket("bm-pocket-6k-pro", "blackmagic.pocket6kpro", "Pocket Cinema Camera 6K Pro", true),
		studio("bm-studio-4k-plus-g2", "blackmagic.studio4kplusg2", "Studio Camera 4K Plus G2", false),
		studio("bm-studio-4k-pro-g2", "blackmagic.studio4kprog2", "Studio Camera 4K Pro G2", true),
	}
}

func pocket(id, alias core.ModelID, name string, hasND bool) camerakit.Planned {
	return camerakit.Planned{
		Model: core.ModelInfo{
			ID: id, Aliases: []core.ModelID{alias}, Brand: "Blackmagic Design", Name: name,
			Transports: []core.Transport{core.TransportUSBEthernet}, DefaultPort: bm.DefaultPort,
			Auth: core.AuthNone, Support: core.SupportPlanned, InterchangeableLens: true,
			Notes: "API REST Blackmagic Camera Control via adaptateur USB-C Ethernet ; le Bluetooth n'est pas encore pris en charge et la version de firmware minimale reste à vérifier.",
		},
		Specs: func(cfg core.CameraConfig) []core.ControlSpec { return pocketSpecs(cfg.PowerZoom, hasND) },
	}
}

func studio(id, alias core.ModelID, name string, hasND bool) camerakit.Planned {
	return camerakit.Planned{
		Model: core.ModelInfo{
			ID: id, Aliases: []core.ModelID{alias}, Brand: "Blackmagic Design", Name: name,
			Transports: []core.Transport{core.TransportEthernet}, DefaultPort: bm.DefaultPort,
			Auth: core.AuthNone, Support: core.SupportPlanned, InterchangeableLens: true,
			Notes: "API REST Blackmagic Camera Control sur le port Ethernet, WebSocket pour les événements ; plages à vérifier sur la caméra réelle.",
		},
		Specs: func(cfg core.CameraConfig) []core.ControlSpec { return studioSpecs(cfg.PowerZoom, hasND) },
	}
}

// shutterSpeeds are the speeds declared until the models are checked.
// TODO(verify) per model and frame rate.
var shutterSpeeds = []string{"1/24", "1/25", "1/30", "1/48", "1/50", "1/60", "1/100", "1/120", "1/250", "1/500", "1/1000", "1/2000"}

// Pocket cameras: no color bars output, ISO instead of gain.
func pocketSpecs(powerZoom, hasND bool) []core.ControlSpec {
	s := lens(powerZoom)
	s = append(s, bm.ISOSpec())
	s = append(s, bm.ShutterSpecs(shutterSpeeds...)...)
	if hasND {
		s = append(s, bm.NDSpec())
	}
	s = append(s, bm.WhiteSpecs()...)
	s = append(s, color()...)
	return append(s, status()...)
}

// Studio cameras: gain in dB, color bars, optional built-in ND.
func studioSpecs(powerZoom, hasND bool) []core.ControlSpec {
	s := lens(powerZoom)
	// TODO(verify): recent firmware also exposes ISO on Studio cameras.
	s = append(s, bm.GainSpec())
	s = append(s, bm.ShutterSpecs(shutterSpeeds...)...)
	if hasND {
		s = append(s, bm.NDSpec())
	}
	s = append(s, bm.WhiteSpecs()...)
	s = append(s, color()...)
	s = append(s, speckit.Toggle(core.ColorBars)) // TODO(verify): /colorBars endpoint availability
	return append(s, status()...)
}

func lens(powerZoom bool) []core.ControlSpec {
	var s []core.ControlSpec
	for _, l := range bm.LensSpecs() {
		if l.ID != core.Zoom || powerZoom {
			s = append(s, l)
		}
	}
	return s
}

func color() []core.ControlSpec {
	return append(bm.WheelSpecs(false),
		speckit.Range(core.Contrast, 0, 2, 0.01, ""),                                                               // TODO(verify)
		speckit.Range(core.Saturation, 0, 2, 0.01, ""),                                                             // TODO(verify)
		speckit.Range(core.Hue, -1, 1, 0.01, ""),                                                                   // TODO(verify)
		speckit.Enum(core.Detail, speckit.Labeled("off", "Off", "low", "Low", "medium", "Medium", "high", "High")), // TODO(verify): sharpening levels
	)
}

func status() []core.ControlSpec {
	return []core.ControlSpec{bm.RecordSpec(), speckit.ReadOnly(speckit.Toggle(core.Tally))}
}
