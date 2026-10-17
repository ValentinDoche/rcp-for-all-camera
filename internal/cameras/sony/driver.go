// Package sony declares the Sony camera modules (cinema line and PTZ) and
// the controls each model exposes.
//
// Intended protocol: Sony network control (web remote / Camera Remote API,
// user and password) over Ethernet or Wi-Fi. Until it is written every
// model is a camerakit.Planned module; the protocol will then go in a
// shared base here, with one module package per model (sony/fx6...).
package sony

import (
	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/camerakit"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// Modules returns the Sony camera modules, in display order.
func Modules() []core.Module {
	var out []core.Module
	for _, m := range models() {
		out = append(out, camerakit.Planned{Model: m, Specs: specsOf(m.ID)})
	}
	return out
}

func models() []core.ModelInfo {
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

func model(id, name string, auth core.AuthMode, notes string, tr ...core.Transport) core.ModelInfo {
	return core.ModelInfo{
		ID: core.ModelID("sony-" + id), Brand: "Sony", Name: name,
		Transports: tr, DefaultPort: 80, // TODO(verify): default port per model
		Auth: auth, Support: core.SupportPlanned, Notes: notes,
	}
}

// specsOf returns the controls of a model. The table is static: a missing
// model is a programming error caught by the tests.
func specsOf(id core.ModelID) func(core.CameraConfig) []core.ControlSpec {
	o := map[core.ModelID]opts{
		"sony-generic": {},
		"sony-fx3":     {iso: true},
		"sony-fx6":     {iso: true, gain: true, shutterAngle: true, nd: true, masterBlack: true, bars: true, zoom: true},
		"sony-fx9":     {iso: true, gain: true, shutterAngle: true, nd: true, masterBlack: true, bars: true, zoom: true},
		"sony-fr7":     {iso: true, nd: true, ptz: true, zoom: true},
	}[id]
	return func(core.CameraConfig) []core.ControlSpec { return family(o) }
}
