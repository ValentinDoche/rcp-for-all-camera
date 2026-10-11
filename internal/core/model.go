package core

import (
	"fmt"
	"net"
	"strings"
)

// ModelID identifies a camera model: "<driver>.<model>", e.g. "sony.fx6".
type ModelID string

// Transport is the physical link used to reach the camera.
type Transport string

const (
	TransportEthernet    Transport = "ethernet"
	TransportWiFi        Transport = "wifi"
	TransportUSBEthernet Transport = "usbEthernet" // USB-C to Ethernet dongle on the camera
)

// AuthMode says whether the camera's control API needs credentials.
type AuthMode string

const (
	AuthNone     AuthMode = "none"
	AuthOptional AuthMode = "optional"
	AuthRequired AuthMode = "required"
)

// SupportLevel tells the user how far the driver is for this model.
type SupportLevel string

const (
	SupportPlanned      SupportLevel = "planned"      // declared, no control yet
	SupportExperimental SupportLevel = "experimental" // works, not verified on hardware
	SupportSupported    SupportLevel = "supported"    // verified on hardware
)

// ModelInfo describes a camera model a driver can control.
type ModelInfo struct {
	ID          ModelID      `json:"id"`
	Brand       string       `json:"brand"`
	Name        string       `json:"name"`
	Driver      string       `json:"driver"`
	Transports  []Transport  `json:"transports"`
	DefaultPort int          `json:"defaultPort"`
	Auth        AuthMode     `json:"auth"`
	Support     SupportLevel `json:"support"`
	Notes       string       `json:"notes,omitempty"`
}

// Check reports whether the model declaration is well formed.
func (m ModelInfo) Check() error {
	switch {
	case m.Driver == "" || !strings.HasPrefix(string(m.ID), m.Driver+".") || len(m.ID) <= len(m.Driver)+1:
		return fmt.Errorf("model %q: id must be %q + name", m.ID, m.Driver+".")
	case m.Brand == "" || m.Name == "":
		return fmt.Errorf("model %s: brand and name are required", m.ID)
	case len(m.Transports) == 0:
		return fmt.Errorf("model %s: no transport", m.ID)
	case m.DefaultPort < 0 || m.DefaultPort > 65535:
		return fmt.Errorf("model %s: bad default port %d", m.ID, m.DefaultPort)
	}
	switch m.Auth {
	case AuthNone, AuthOptional, AuthRequired:
	default:
		return fmt.Errorf("model %s: bad auth mode %q", m.ID, m.Auth)
	}
	switch m.Support {
	case SupportPlanned, SupportExperimental, SupportSupported:
	default:
		return fmt.Errorf("model %s: bad support level %q", m.ID, m.Support)
	}
	return nil
}

// CameraConfig is what the user enters for one camera.
type CameraConfig struct {
	Model     ModelID   `json:"model"`
	Name      string    `json:"name"`
	Host      string    `json:"host"`
	Port      int       `json:"port,omitempty"` // 0 = model default
	Transport Transport `json:"transport,omitempty"`
	Username  string    `json:"username,omitempty"`
	// Password is never serialized; the config package stores it in the OS
	// secret store.
	Password string `json:"-"`
}

const (
	maxNameLen = 64
	maxCredLen = 128
)

// ValidateConfig checks a user-entered config against its model. It is the
// boundary check for anything coming from the GUI or a config file.
func ValidateConfig(m ModelInfo, c CameraConfig) error {
	bad := func(format string, a ...any) error {
		return fmt.Errorf("%s: %s: %w", m.ID, fmt.Sprintf(format, a...), ErrInvalidConfig)
	}
	if c.Model != m.ID {
		return bad("config is for model %q", c.Model)
	}
	if len(c.Name) > maxNameLen || hasControlChars(c.Name) {
		return bad("name must be at most %d printable characters", maxNameLen)
	}
	if !validHost(c.Host) {
		return bad("host must be an IP address or a host name")
	}
	if c.Port < 0 || c.Port > 65535 {
		return bad("port %d out of range", c.Port)
	}
	if c.Transport != "" && !hasTransport(m, c.Transport) {
		return bad("transport %q not available on this model", c.Transport)
	}
	if len(c.Username) > maxCredLen || len(c.Password) > maxCredLen ||
		hasControlChars(c.Username) || hasControlChars(c.Password) {
		return bad("credentials must be at most %d printable characters", maxCredLen)
	}
	if m.Auth == AuthRequired && c.Username == "" {
		return bad("username is required")
	}
	return nil
}

func hasTransport(m ModelInfo, t Transport) bool {
	for _, x := range m.Transports {
		if x == t {
			return true
		}
	}
	return false
}

func hasControlChars(s string) bool {
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return true
		}
	}
	return false
}

// validHost accepts an IPv4/IPv6 literal or an RFC 1123 host name. URLs,
// paths, ports and spaces are rejected so the value can be used safely to
// build addresses later.
func validHost(h string) bool {
	if h == "" || len(h) > 253 {
		return false
	}
	if net.ParseIP(h) != nil {
		return true
	}
	for _, label := range strings.Split(strings.TrimSuffix(h, "."), ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for i := 0; i < len(label); i++ {
			ch := label[i]
			if !(ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9' || ch == '-') {
				return false
			}
		}
	}
	return true
}
