// Package rcpmod is the SDK for writing RCP For All Camera camera modules
// in Go. A module is compiled to WebAssembly (one <id>.wasm per camera
// model) and loaded by the application from its modules folder:
//
//	package main
//
//	import "github.com/valentindoche/rcp-for-all-camera/sdk/rcpmod"
//
//	func main() {}
//
//	func init() { rcpmod.Register(myModule{}) }
//
// Build it with go run ./cmd/buildmodules, or by hand:
//
//	GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o my-camera.wasm .
//
// A module runs in a sandbox: it reaches its camera only through the Host
// the application lends it. The application calls a module one call at a
// time, so module code needs no lock. docs/modules-camera.html explains the
// whole contract.
package rcpmod

import "time"

// Module is the code that controls one camera model.
type Module interface {
	// Info declares the model. It is static.
	Info() Info
	// New builds a camera for a validated config. It must not do any I/O;
	// connecting is Camera.Connect's job.
	New(cfg Config, host Host) (Camera, error)
}

// Camera is one configured camera.
type Camera interface {
	// Controls is static for a given config and needs no connection.
	Controls() []ControlSpec
	// Connect starts connecting. The camera then reports its state with a
	// Status event, and keeps reconnecting by itself until Close.
	Connect() error
	// Close stops everything; the camera is not used afterwards.
	Close()
	// Get returns a control's current value.
	Get(id string) (Value, error)
	// Set validates v against the control's spec before any I/O, then
	// writes it. Changes are reported with Emit.
	Set(id string, v Value) error
}

// Host is the application, as a module sees it. Every network call goes
// to the address and port the user entered for this camera.
type Host interface {
	// HTTP sends one request to the camera; path starts with "/".
	HTTP(method, path string, body []byte) (status int, resp []byte, err error)
	// OpenWebSocket connects to ws://<camera><path>. onMessage receives each
	// text message, onClose the reason the socket closed (nil after Close).
	// The host keeps the socket alive with pings.
	OpenWebSocket(path string, onMessage func([]byte), onClose func(error)) (WebSocket, error)
	// After calls fn once after d, unless cancelled.
	After(d time.Duration, fn func()) (cancel func())
	// Emit sends an event to the application.
	Emit(e Event)
	// Log writes a line in the application log.
	Log(level Level, msg string)
}

// WebSocket is an open socket to the camera.
type WebSocket interface {
	Send(text []byte) error
	Close()
}

// Level is a log level.
type Level int

const (
	LevelDebug Level = -4
	LevelInfo  Level = 0
	LevelWarn  Level = 4
	LevelError Level = 8
)

var registered Module

// Register makes m the module of this program. Call it once, from an
// init function of the main package.
func Register(m Module) {
	if registered != nil {
		panic("rcpmod.Register called twice")
	}
	registered = m
}

// Planned is the module of a camera model whose protocol is not written
// yet: it declares the model and its controls; connecting and writing
// answer ErrNotImplemented.
type Planned struct {
	Model Info
	// Specs returns the controls for a config (a power zoom adds a zoom).
	Specs func(cfg Config) []ControlSpec
	// Page lays out the camera page (nil = StandardPage).
	Page func(cfg Config) Page
}

// Info is the model declaration.
func (p Planned) Info() Info { return p.Model }

// New builds a camera without protocol.
func (p Planned) New(cfg Config, _ Host) (Camera, error) {
	page := StandardPage()
	if p.Page != nil {
		page = p.Page(cfg)
	}
	return plannedCamera{specs: NewSpecs(p.Specs(cfg)), page: page}, nil
}

type plannedCamera struct {
	specs Specs
	page  Page
}

func (c plannedCamera) Page() Page { return c.page }

func (c plannedCamera) Controls() []ControlSpec { return c.specs.All() }
func (c plannedCamera) Connect() error          { return ErrNotImplemented }
func (c plannedCamera) Close()                  {}

func (c plannedCamera) Get(id string) (Value, error) {
	if _, err := c.specs.Find(id); err != nil {
		return Value{}, err
	}
	return Value{}, ErrNotConnected
}

func (c plannedCamera) Set(id string, v Value) error {
	if _, err := c.specs.Validate(id, v); err != nil {
		return err
	}
	return ErrNotImplemented
}
