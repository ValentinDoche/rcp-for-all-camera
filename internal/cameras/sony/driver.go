// Package sony declares the Sony cameras (cinema line and PTZ) and the
// controls each model exposes.
//
// Intended protocol: Sony network control (web remote / Camera Remote API,
// user and password) over Ethernet or Wi-Fi. Until the protocol is written
// every model uses camerakit.Placeholder.
package sony

import (
	"fmt"

	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/camerakit"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// Driver implements core.Driver for Sony cameras.
type Driver struct{}

var _ core.Driver = Driver{}

func (Driver) Name() string { return "sony" }

func (Driver) Models() []core.ModelInfo {
	return []core.ModelInfo{
		model("generic", "Autre modèle Sony", core.AuthRequired,
			"Interface web distante Sony (utilisateur et mot de passe) ; modèle exact et jeu de commandes à vérifier.",
			core.TransportEthernet),
		model("fx3", "FX3", core.AuthOptional,
			"Camera Remote via Wi-Fi ; un appairage est probablement nécessaire et l'authentification reste à vérifier.",
			core.TransportWiFi),
		model("fx6", "FX6", core.AuthRequired,
			"Interface web distante Sony en Wi-Fi ou via adaptateur USB Ethernet ; les commandes disponibles sont à vérifier.",
			core.TransportWiFi, core.TransportUSBEthernet),
		model("fx9", "FX9", core.AuthRequired,
			"Interface web distante Sony sur le port Ethernet ; les commandes disponibles sont à vérifier.",
			core.TransportEthernet),
		model("fr7", "FR7", core.AuthRequired,
			"Contrôle réseau Sony avec commandes PTZ sur le port Ethernet ; le protocole PTZ exact est à vérifier.",
			core.TransportEthernet),
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

func model(id, name string, auth core.AuthMode, notes string, tr ...core.Transport) core.ModelInfo {
	return core.ModelInfo{
		ID: core.ModelID("sony." + id), Brand: "Sony", Name: name, Driver: "sony",
		Transports: tr, DefaultPort: 80, // TODO(verify): default port per model
		Auth: auth, Support: core.SupportPlanned, Notes: notes,
	}
}

func specsFor(id core.ModelID) ([]core.ControlSpec, error) {
	switch id {
	case "sony.generic":
		return family(opts{}), nil
	case "sony.fx3":
		return family(opts{iso: true}), nil
	case "sony.fx6", "sony.fx9":
		return family(opts{iso: true, gain: true, shutterAngle: true, nd: true, masterBlack: true, bars: true, zoom: true}), nil
	case "sony.fr7":
		return family(opts{iso: true, nd: true, ptz: true, zoom: true}), nil
	}
	return nil, fmt.Errorf("%q: %w", id, core.ErrUnknownModel)
}
