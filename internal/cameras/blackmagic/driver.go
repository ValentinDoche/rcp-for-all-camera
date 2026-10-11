// Package blackmagic declares the Blackmagic Design cameras (Pocket Cinema
// Camera and Studio Camera families) and the controls each model exposes.
//
// Intended protocol: Blackmagic Camera Control REST API over HTTP (port 80),
// no authentication. Studio cameras use their Ethernet port; Pocket cameras
// need a USB-C to Ethernet adapter (Bluetooth is the classic Pocket link and
// is not a supported transport yet). Until the protocol is written every
// model uses camerakit.Placeholder.
package blackmagic

import (
	"fmt"

	"github.com/valentindoche/rcp-for-all-camera/internal/cameras/camerakit"
	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// Driver implements core.Driver for Blackmagic Design cameras.
type Driver struct{}

var _ core.Driver = Driver{}

func (Driver) Name() string { return "blackmagic" }

func (Driver) Models() []core.ModelInfo {
	return []core.ModelInfo{
		pocket("pocket4k", "Pocket Cinema Camera 4K"),
		pocket("pocket6kg2", "Pocket Cinema Camera 6K G2"),
		pocket("pocket6kpro", "Pocket Cinema Camera 6K Pro"),
		studio("studio4kplusg2", "Studio Camera 4K Plus G2"),
		studio("studio4kprog2", "Studio Camera 4K Pro G2"),
		studio("studio6kpro", "Studio Camera 6K Pro"),
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

func pocket(id, name string) core.ModelInfo {
	return core.ModelInfo{
		ID: core.ModelID("blackmagic." + id), Brand: "Blackmagic Design", Name: name, Driver: "blackmagic",
		Transports: []core.Transport{core.TransportUSBEthernet}, DefaultPort: 80,
		Auth: core.AuthNone, Support: core.SupportPlanned,
		Notes: "API REST Blackmagic Camera Control via adaptateur USB-C Ethernet ; le Bluetooth n'est pas encore pris en charge et la version de firmware minimale reste à vérifier.",
	}
}

func studio(id, name string) core.ModelInfo {
	return core.ModelInfo{
		ID: core.ModelID("blackmagic." + id), Brand: "Blackmagic Design", Name: name, Driver: "blackmagic",
		Transports: []core.Transport{core.TransportEthernet}, DefaultPort: 80,
		Auth: core.AuthNone, Support: core.SupportPlanned,
		Notes: "API REST Blackmagic Camera Control sur le port Ethernet, WebSocket pour les événements ; plages à vérifier sur la caméra réelle.",
	}
}

func specsFor(id core.ModelID) ([]core.ControlSpec, error) {
	switch id {
	case "blackmagic.pocket4k":
		return pocketSpecs(false), nil
	case "blackmagic.pocket6kg2":
		return pocketSpecs(false), nil
	case "blackmagic.pocket6kpro":
		return pocketSpecs(true), nil
	case "blackmagic.studio4kplusg2":
		return studioSpecs(false), nil
	case "blackmagic.studio4kprog2", "blackmagic.studio6kpro":
		return studioSpecs(true), nil
	}
	return nil, fmt.Errorf("%q: %w", id, core.ErrUnknownModel)
}
