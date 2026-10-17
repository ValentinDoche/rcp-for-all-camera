package core

import (
	"fmt"
	"log/slog"
	"net"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ModelID identifies a camera module, hence a camera model: lowercase
// words joined by hyphens, starting with the brand ("bm-studio-6k-pro",
// "sony-fx6", "demo"). It is saved in rig.json, so it should not change
// once released: a camera saved under an unknown id is dropped at startup.
type ModelID string

// maxModelIDLen bounds model ids and aliases.
const maxModelIDLen = 64

// CheckModelID reports whether id is a well-formed module id. The error
// never echoes the id.
func CheckModelID(id ModelID) error {
	if id == "" || len(id) > maxModelIDLen {
		return fmt.Errorf("model id must be 1 to %d characters: %w", maxModelIDLen, ErrUnknownModel)
	}
	prevHyphen := true // no leading hyphen
	for i := 0; i < len(id); i++ {
		c := id[i]
		switch {
		case c >= 'a' && c <= 'z' || c >= '0' && c <= '9':
			prevHyphen = false
		case c == '-' && !prevHyphen:
			prevHyphen = true
		default:
			return fmt.Errorf("model id must be lowercase words joined by single hyphens: %w", ErrUnknownModel)
		}
	}
	if prevHyphen {
		return fmt.Errorf("model id must not end with a hyphen: %w", ErrUnknownModel)
	}
	return nil
}

// Transport is a physical link the camera supports. It is informational
// (shown to the user); control always goes over IP, whatever the link.
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
	ID          ModelID
	Brand       string
	Name        string
	Transports  []Transport
	DefaultPort int
	Auth        AuthMode
	Support     SupportLevel
	Notes       string
	// InterchangeableLens: the lens is the user's choice, so whether it has
	// a power zoom is a per-camera setting (CameraConfig.PowerZoom). Models
	// with a built-in zoom lens (PTZ) leave it false.
	InterchangeableLens bool
	// NoAddress: the camera is not on the network (the demo camera); the
	// config needs no host or port.
	NoAddress bool
}

// Check reports whether the model declaration is well formed.
func (m ModelInfo) Check() error {
	if err := CheckModelID(m.ID); err != nil {
		return fmt.Errorf("model %q: %w", m.ID, err)
	}
	switch {
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
	Model    ModelID
	Name     string
	Host     string
	Port     int // 0 = model default
	Username string
	// PowerZoom: the mounted lens has a motorised zoom. Only read for models
	// with an InterchangeableLens; without it the camera declares no zoom.
	PowerZoom bool
	// Password is never serialized; the config package stores it in the OS
	// secret store.
	Password string
	// BindIP is the local address drivers must dial from ("" = let the OS
	// choose). It is an app-wide setting filled in by the app, not per camera.
	BindIP string
}

// redacted is CameraConfig without its password, for logs and %v output.
type redacted struct {
	Model       ModelID
	Name, Host  string
	Port        int
	BindIP      string
	Username    string
	HasPassword bool
}

func (c CameraConfig) redacted() redacted {
	return redacted{c.Model, c.Name, c.Host, c.Port, c.BindIP, c.Username, c.Password != ""}
}

// String, GoString and LogValue keep the password out of fmt verbs and slog,
// so a CameraConfig can be logged or wrapped in an error without leaking it.
func (c CameraConfig) String() string { return fmt.Sprintf("%+v", c.redacted()) }

// GoString is String for the %#v verb.
func (c CameraConfig) GoString() string { return "core.CameraConfig" + c.String() }

// LogValue logs the configuration without its password.
func (c CameraConfig) LogValue() slog.Value {
	return slog.GroupValue(
		slog.String("model", string(c.Model)), slog.String("name", c.Name),
		slog.String("host", c.Host), slog.Int("port", c.Port),
		slog.String("bindIp", c.BindIP), slog.String("username", c.Username),
		slog.Bool("hasPassword", c.Password != ""),
	)
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
	if len(c.Name) > maxNameLen || !printable(c.Name) {
		return bad("name must be at most %d printable characters", maxNameLen)
	}
	if !m.NoAddress && !validHost(c.Host) {
		return bad("host must be an IP address or a host name")
	}
	if c.Port < 0 || c.Port > 65535 {
		return bad("port %d out of range", c.Port)
	}
	if len(c.Username) > maxCredLen || len(c.Password) > maxCredLen ||
		!printable(c.Username) || !printable(c.Password) {
		return bad("credentials must be at most %d printable characters", maxCredLen)
	}
	if m.Auth == AuthRequired && c.Username == "" {
		return bad("username is required")
	}
	return nil
}

// printable reports whether s is valid UTF-8 without control or format
// characters (C0/C1 controls, bidi overrides, zero-width marks), which could
// spoof what the GUI shows or corrupt a protocol line.
func printable(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return false
		}
	}
	return true
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
