# Spec: RCP For All Camera

Status: DRAFT, awaiting owner review. Source of intent: [docs/intent/rcp-for-all-camera.md](docs/intent/rcp-for-all-camera.md)

## Objective

Make a Blackmagic **ATEM Micro Camera Panel** usable as a universal camera control panel on a PC with **no ATEM switcher**.

- The panel connects to the PC over **USB**.
- The cameras connect over the network (Ethernet, Wi-Fi or a USB-C Ethernet dongle depending on the body). Supported models (owner list, 2026-10-11): Blackmagic Pocket 4K, Pocket 6K G2, Pocket 6K Pro, Studio 4K Plus G2, Studio 4K Pro G2, Studio 6K Pro; Sony FX3 (Wi-Fi), FX6 (Wi-Fi or USB-C Ethernet), FX9, FR7, other Sony (user/password); Panasonic AW-UE100, AW-UE150, AW-UE160, other Panasonic.
- Pressing panel button 1-8 does two things at once: it selects which camera the panel's controls drive, and it routes that source to an **aux output** on an external router/mixer (Videohub, ATEM or vMix).
- The panel's controls drive every control each camera exposes over the network.
- The project is open source and modular: surface, camera and router code are separate modules, so new devices can be added later.

**Primary user:** the owner. **Secondary user:** anyone with the same panel and non-ATEM gear.

### Non-goals

- Not a video switcher; no video or audio handling.
- No cameras beyond the list above in v1 (the camera module system must allow more: see docs/adding-a-camera.md).
- No installer, licensing, cloud or remote access.

## Assumptions (correct me before planning)

1. Host OS is **Windows 11**. Linux/macOS are nice to have, not required.
2. Language is **Go** (decided by the owner, for easy packaging into a single executable). Consequences:
   - Videohub (TCP text) and vMix (HTTP/TCP) are simple protocols and are implemented directly.
   - The ATEM protocol has no official Go library; options are a community Go package or implementing the open, community-documented protocol ourselves. This is a risk for `router-atem` (see Open Questions).
   - Sony's Camera Remote SDK is C++; from Go it would need cgo. Prefer a plain network protocol (e.g. VISCA over IP, UDP) where the cameras support it.
   - Reading the panel over USB from Go may need cgo (HID or libusb). The spike decides this.
3. *(Resolved by the spike: HID with challenge/response, see Open Question 1.)* The panel's USB protocol was **not known** at first. It may be a network-over-USB link carrying the ATEM protocol, or HID. The first task is a feasibility spike (see Open Questions). This decides how the "Surface" module is built.
4. Blackmagic Studio 6K Pro is controlled through Blackmagic's **REST/WebSocket camera control API** over Ethernet.
5. Sony FX6/FX9 are controlled over **IP/Ethernet only (no LANC)**. The exact network interface and the controls it exposes **need verification** per camera.
6. Configuration is done in a **GUI** (not a hand-edited file). The GUI stores its settings in a config file that can still be exported and shared. The GUI is a **native Fyne** desktop app (decided 2026-10-11: the owner refused a web UI, so no Wails/WebView), producing a single `.exe` (cgo, MinGW-w64). Light and dark themes. Every parameter a camera exposes is controllable from the GUI. The core runs without the GUI (headless mode) so it can be tested and run as a service.
7. Router/mixer protocols: Videohub (TCP text protocol, port 9990), ATEM (UDP, see assumption 2), vMix (HTTP or TCP API).

## Capability Map

The request bundles several independently testable capabilities, so it is split into modules with stable ids. Each gets its own module spec (`SPEC-<id>.md`) after this map is approved.

| Module id | Responsibility | Depends on |
|---|---|---|
| `core` | Device registry, control model (normalized control names/values), config loading, event bus, logging | none |
| `surface-micro-panel` | Talk to the ATEM Micro Camera Panel over USB; emit normalized control events (iris, gain, joystick, buttons 1-8); drive panel LEDs/state | `core` |
| `camera-blackmagic` | Blackmagic Studio 6K Pro control over Ethernet; maps normalized controls to REST/WebSocket calls and reads back state | `core` |
| `camera-sony` | Sony FX6/FX9 control over network; maps the supported subset of normalized controls; declares capability gaps | `core` |
| `router-videohub` | Aux/route control and state for Blackmagic Videohub | `core` |
| `router-atem` | Aux output control on an ATEM switcher | `core` |
| `router-vmix` | Aux/output control on vMix | `core` |
| `app` | Wires modules together, button-to-camera-and-aux mapping, headless runner | all above |
| `gui` | Native Fyne desktop GUI: panel detection, assign cameras to slots 1-8 with their model, control every camera parameter, add/edit routers, button mapping, live status | `core`, `app` |

Build order: `core` → `surface-micro-panel`, `camera-blackmagic` → `camera-sony`, `router-videohub` → `router-atem`, `router-vmix` → `app` → `gui`

Interfaces live in `core`: a `Camera` interface (capabilities, set/get control, connect state), a `Router` interface (list sources, route source to aux) and a `Surface` interface (control events, state feedback).

## Commands

Planned (to be confirmed once the project is scaffolded):

```
Build (headless): go build ./cmd/rcp
Build (GUI):      CGO_ENABLED=1 go build -ldflags "-H windowsgui" -o rcp.exe ./cmd/rcp
Test:             go test ./... -race -cover
Lint:             golangci-lint run
Run (headless):   ./rcp --config rig.json
```

## Project Structure

```
cmd/rcp/                       → Headless entry point
internal/core/                 → Interfaces, control model, config, event bus
internal/surfaces/micropanel/  → ATEM Micro Camera Panel driver
internal/cameras/blackmagic/   → Blackmagic camera driver
internal/cameras/sony/         → Sony FX6/FX9 IP driver
internal/routers/videohub|atem|vmix → Router drivers
internal/app/                  → Wiring, button/control mapping, headless runner
internal/gui/                  → Fyne native GUI
testdata/                      → Fake-device fixtures
docs/                          → Intent, protocol notes, per-module docs
SPEC.md, SPEC-<module>.md      → This spec and module specs
```

## Code Style

Idiomatic Go: `gofmt`, small interfaces defined where they are used, errors returned and wrapped with `%w`, `context.Context` on anything that does I/O.

```go
// internal/core/camera.go
package core

type ControlID string

const (
	Iris         ControlID = "iris"
	Gain         ControlID = "gain"
	ISO          ControlID = "iso"
	ND           ControlID = "nd"
	WhiteBalance ControlID = "whiteBalance"
	Focus        ControlID = "focus"
	Record       ControlID = "record"
)

type Camera interface {
	ID() string
	Capabilities() map[ControlID]struct{}
	Connect(ctx context.Context) error
	SetControl(ctx context.Context, c ControlID, v float64) error
	Subscribe(fn func(c ControlID, v float64)) (cancel func())
}
```

- Names: standard Go conventions; package names short and lowercase.
- A camera that cannot do a control must not list it in `Capabilities()`; the app never silently ignores a failed command.
- Anything needing cgo (USB/HID) stays behind the `Surface` interface and a build tag, so the rest of the project builds with `CGO_ENABLED=0`.

## Testing Strategy

- **Framework:** Go standard `testing` with `testify` for assertions.
- **Unit tests:** normalization and mapping (panel value ↔ camera value), config parsing, button-to-camera/aux mapping.
- **Contract tests:** one shared test suite that every `Camera`, `Router` and `Surface` implementation must pass.
- **Integration tests:** drivers against **fake devices** (local mock HTTP/WebSocket/TCP servers) so CI needs no hardware.
- **Manual hardware checklist:** per camera/panel, kept in `docs/hardware-checklist.md`, run before each release.
- Coverage target: 80% on `core` and mapping logic; drivers covered by contract + integration tests.

## Boundaries

- **Always:** run tests before commit; keep protocol details inside the driver module; log every command sent to a device at debug level; handle device disconnect and reconnect without crashing.
- **Ask first:** adding a runtime dependency; changing the `Camera`/`Router`/`Surface` interfaces; changing the config file format; sending any command that could change recording or output on live gear during tests.
- **Never:** commit secrets or device passwords; send commands to a camera that is not in the configured rig; remove or skip failing tests without approval; reverse-engineer in a way that violates a device license (use official docs/SDKs where they exist).

## Control Behaviour Requirements

Behaviour the owner asked for. It applies to the `app` mapping layer and the surface driver.

- **CB-1 Iris pickup (soft takeover)**, requested 2026-10-11. The iris lever is an
  absolute physical position that the software cannot move. After switching to
  another camera, moving the lever must **not** change that camera's iris until
  the lever reaches (crosses) the camera's current iris value. Only then does the
  lever take control.
  - Example: the camera is at F10 and the lever is at F4. Nothing happens while the
    lever moves around F4. Control starts once the lever passes F10.
  - The panel should show that it is waiting for pickup. The IRIS display always
    shows the camera's real value; a hint could use the iris bar or the IRIS/MB ACTIVE LED.
  - The same rule applies to the **master black ring** (also absolute, ±4096).
  - The 9 knobs (report 0x0E) are encoders whose position the panel keeps itself.
    Prefer writing feature 0x0E to resync them to the camera value (untested),
    and fall back to pickup if that isn't possible.

## Success Criteria

1. With the panel plugged in over USB and no ATEM switcher, the program detects the panel and shows it as connected.
2. Pressing button N selects camera N **and** routes source N to the configured aux output within 200 ms on a Videohub, an ATEM and vMix (each one verified separately).
3. For the Blackmagic Studio 6K Pro, every control the camera's network API exposes (iris, gain/ISO, ND, white balance, focus, color correction, record) can be driven from the panel, and the panel reflects state changes made on the camera.
4. For the FX6 and FX9, every control exposed by Sony's IP interface can be driven from the panel (no LANC); unsupported controls are reported clearly (not silent).
5. All configuration (cameras, routers, button mapping, control mapping) can be done from the GUI without editing a file, and the app ships as a single Windows `.exe`.
6. A camera or router going offline does not crash the program; it reconnects automatically and the GUI shows the state.
7. Adding a new camera type requires only a new module that implements the `Camera` interface plus a registration; no change to `core` or other modules.
8. All tests pass in CI without hardware attached.

## Open Questions

1. ~~**Panel USB protocol**~~ **Resolved 2026-10-11 (GO):** authenticated vendor HID, works with no switcher, driven from pure Go without cgo. See [docs/reports/2026-10-11-micro-camera-panel-usb-spike.md](docs/reports/2026-10-11-micro-camera-panel-usb-spike.md) and [docs/atem-micro-camera-panel-protocol.md](docs/atem-micro-camera-panel-protocol.md).
2. **Sony IP interface:** which network interface supports FX6 and FX9 with the broadest control set (VISCA over IP, Camera Remote SDK via cgo, a REST/web API)? Does it cover iris, ISO/gain, ND, white balance and focus on both bodies? Pure Go (VISCA/UDP, REST) is preferred; the Camera Remote SDK would force cgo and a C++ dependency.
3. **ATEM from Go:** use a community Go library or implement the protocol ourselves for `router-atem`? Needs a quick maturity check.
4. **Controls to physical mapping:** which panel knob/joystick maps to which control, and which controls need a "page" or modifier because the panel has fewer physical controls than the cameras expose? (Configurable in the GUI, but a sensible default is needed.)
5. **Aux model:** one aux output for all cameras, or one aux per camera group? How are button numbers mapped to router inputs (configured table assumed)?
6. ~~**GUI framework**~~ **Resolved 2026-10-11:** native Fyne GUI (Wails was tried and removed: the owner wants no web frontend).
7. **License:** MIT, Apache-2.0 or other for the open-source release?
