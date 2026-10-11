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
		model("awue150", "AW-UE150", "Commandes HTTP/CGI Panasonic AW ; les plages de gain, d'obturateur et de preset sont à vérifier."),
		model("awue160", "AW-UE160", "Commandes HTTP/CGI Panasonic AW ; les plages de gain, d'obturateur et de preset sont à vérifier."),
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
	switch id {
	case "panasonic.generic":
		return family(opts{}), nil
	case "panasonic.awue100":
		return family(opts{ptz: true}), nil
	case "panasonic.awue150", "panasonic.awue160":
		// TODO(verify): UE100 may also have a built-in ND filter.
		return family(opts{ptz: true, nd: true}), nil
	}
	return nil, fmt.Errorf("%q: %w", id, core.ErrUnknownModel)
}
