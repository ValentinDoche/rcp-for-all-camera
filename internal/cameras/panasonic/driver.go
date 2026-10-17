// Package panasonic declares the Panasonic PTZ camera modules and the
// controls each model exposes.
//
// Intended protocol: Panasonic AW HTTP/CGI commands (port 80, optional basic
// authentication). Until it is written every model is a camerakit.Planned
// module; the protocol will then go in a shared base here, with one module
// package per model.
package panasonic

import (
	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/camerakit"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// Modules returns the Panasonic camera modules, in display order.
func Modules() []core.Module {
	var out []core.Module
	for _, m := range models() {
		out = append(out, camerakit.Planned{Model: m, Specs: func(core.CameraConfig) []core.ControlSpec { return specsFor(m.ID) }})
	}
	return out
}

func models() []core.ModelInfo {
	return []core.ModelInfo{
		model("generic", "Autre modèle Panasonic", "Commandes HTTP/CGI Panasonic AW ; le modèle exact et les commandes prises en charge sont à vérifier."),
		model("aw-ue100", "AW-UE100", "Commandes HTTP/CGI Panasonic AW ; la présence d'un filtre ND et les plages sont à vérifier."),
		model("aw-ue150", "AW-UE150", "La liste des paramètres vient de la référence SKAARHOJ core-panasonic-ptz ; le protocole (Panasonic AW HTTP/CGI) reste à écrire."),
		model("aw-ue160", "AW-UE160", "La liste des paramètres vient de la référence SKAARHOJ core-panasonic-ptz ; le protocole (Panasonic AW HTTP/CGI) reste à écrire."),
	}
}

// model declares a Panasonic model. id is the module id without its brand
// ("aw-ue150").
func model(id, name, notes string) core.ModelInfo {
	return core.ModelInfo{
		ID: core.ModelID("panasonic-" + id), Brand: "Panasonic", Name: name,
		Transports: []core.Transport{core.TransportEthernet}, DefaultPort: 80,
		Auth: core.AuthOptional, Support: core.SupportPlanned, Notes: notes,
	}
}

// specsFor returns the controls of a model, guarded ones marked. The table
// is static: an unknown id is a programming error caught by the tests.
func specsFor(id core.ModelID) []core.ControlSpec {
	var specs []core.ControlSpec
	switch id {
	case "panasonic-generic":
		specs = family(opts{})
	case "panasonic-aw-ue100":
		// TODO(verify): UE100 may also have a built-in ND filter.
		specs = family(opts{ptz: true})
	case "panasonic-aw-ue150":
		specs = ue150Specs()
	case "panasonic-aw-ue160":
		specs = ue160Specs()
	default:
		panic("panasonic: no specs for " + string(id))
	}
	for i := range specs {
		if guarded[specs[i].ID] {
			specs[i].Guarded = true
		}
	}
	return specs
}

// guarded are the Panasonic controls that change power, the video format or
// what goes on air (OSD and scene files show on the output; stores overwrite
// camera memory). The operator lock asks before writing them.
var guarded = map[core.ControlID]bool{
	core.ColorBars:                     true,
	"panasonic.power":                  true,
	"panasonic.frequency":              true,
	"panasonic.systemFormat":           true,
	"panasonic.sdiOutputFormat":        true,
	"panasonic.12GSDIOutputFormat":     true,
	"panasonic.hdmiOutputFormat":       true,
	"panasonic.monitorSDIOutputFormat": true,
	"panasonic.osdMenuEnable":          true,
	"panasonic.sceneFile":              true,
	"panasonic.sceneFileStore":         true,
	"panasonic.userFile":               true,
	"panasonic.userFileStore":          true,
	"panasonic.presetStore":            true,
	"panasonic.presetDelete":           true,
}
