// Package panasonic declares the Panasonic PTZ cameras and the controls each
// model exposes.
//
// Intended protocol: Panasonic AW HTTP/CGI commands (port 80, optional basic
// authentication). Until the protocol is written every model uses
// camerakit.Placeholder.
package panasonic

import (
	"fmt"

	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/camerakit"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// Driver implements core.Driver for Panasonic cameras.
type Driver struct{}

var _ core.Driver = Driver{}

func (Driver) Name() string { return "panasonic" }

func (Driver) Models() []core.ModelInfo {
	return []core.ModelInfo{
		model("generic", "Autre modèle Panasonic", "Commandes HTTP/CGI Panasonic AW ; le modèle exact et les commandes prises en charge sont à vérifier."),
		model("awue100", "AW-UE100", "Commandes HTTP/CGI Panasonic AW ; la présence d'un filtre ND et les plages sont à vérifier."),
		model("awue150", "AW-UE150", "La liste des paramètres vient de la référence SKAARHOJ core-panasonic-ptz ; le protocole (Panasonic AW HTTP/CGI) reste à écrire."),
		model("awue160", "AW-UE160", "La liste des paramètres vient de la référence SKAARHOJ core-panasonic-ptz ; le protocole (Panasonic AW HTTP/CGI) reste à écrire."),
	}
}

// New builds a placeholder camera; it does no I/O.
func (Driver) New(m core.ModelInfo, _ core.CameraConfig) (core.Camera, error) {
	specs, err := specsFor(m.ID)
	if err != nil {
		return nil, err
	}
	return camerakit.NewPlaceholder(specs), nil
}

func model(id, name, notes string) core.ModelInfo {
	return core.ModelInfo{
		ID: core.ModelID("panasonic." + id), Brand: "Panasonic", Name: name, Driver: "panasonic",
		Transports: []core.Transport{core.TransportEthernet}, DefaultPort: 80,
		Auth: core.AuthOptional, Support: core.SupportPlanned, Notes: notes,
	}
}

func specsFor(id core.ModelID) ([]core.ControlSpec, error) {
	var specs []core.ControlSpec
	switch id {
	case "panasonic.generic":
		specs = family(opts{})
	case "panasonic.awue100":
		// TODO(verify): UE100 may also have a built-in ND filter.
		specs = family(opts{ptz: true})
	case "panasonic.awue150":
		specs = ue150Specs()
	case "panasonic.awue160":
		specs = ue160Specs()
	default:
		return nil, fmt.Errorf("%q: %w", id, core.ErrUnknownModel)
	}
	for i := range specs {
		if guarded[specs[i].ID] {
			specs[i].Guarded = true
		}
	}
	return specs, nil
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
