# Architecture

The illustrated version, with the path from a panel movement to the camera, is in the
[documentation home](index.html#architecture). This page is the contributor's summary.

```
cmd/rcp ─────────────► internal/gui (Fyne, native) ─────────┐
   │                                                        │ calls
   ├──────────────► internal/surfaces/panel ────────────────┤ (panel controller,
   │                   └─ panelmodel (pure rules)           │  model in panelmodel)
   │                                                        ▼
   ├──────────────► internal/app  Service (use cases) + Host (lifecycle)
   │                   │ ports: Store, Secrets, PanelSources, Surface
   │                   ▼
   │                internal/core  (controls, values, events, models, pages)
   │
   ├──────────────► internal/config          implements app.Store / app.Secrets
   ├──────────────► internal/plugins         implements core.Module for each modules/*.wasm
   │                   └─ wazero sandbox ──► modules/<id>/  (written with sdk/rcpmod)
   ├──────────────► internal/surfaces/micropanel   USB HID driver (Windows)
   ├──────────────► internal/surfaces/panelsim     on-screen panel simulator
   ├──────────────► internal/surfaces/panelsource  USB panels or the simulator
   └──────────────► internal/sysinfo, internal/autostart, internal/i18n
```

## Rules

- `core` depends on nothing. `app` depends on `core` only and declares the ports it needs;
  adapters (`config`, `plugins`, the surfaces, the GUI) implement or call them. `cmd/rcp` wires
  everything and holds no logic. `internal/archtest/dependency_rules_test.go` enforces it.
- A camera model is a WASM module in the `modules` folder, loaded at start. Adding a camera means
  adding a module; see [adding-a-camera.md](adding-a-camera.md) and the
  [Camera modules guide](modules-camera.html). A module imports only `sdk/rcpmod`.
- Modules run in a sandbox (no file system, no environment, network only to their camera,
  memory, time and message limits). The host validates every page, control and event a module
  sends before the rest of the application sees it.
- A camera declares all its controls; its `ui.go` lays out the details page. The GUI renders any
  control it receives, so a new parameter needs no GUI change.
- The panel driver keeps protocol details inside `internal/surfaces/micropanel`. The panel
  controller (`internal/surfaces/panel`) imports `app` and `micropanel`; `app` never imports it.
  Its rules (what the panel shows, what each input does) live in the pure package `panelmodel`.
- `internal/surfaces/panelsim` is a simulated panel; `rcp.exe -sim` or the panel status menu
  selects it ([panelsim.md](panelsim.md)).
- Passwords live only in the secrets store, never in `rig.json`, logs or the GUI.

## Status (2026-10-18, 0.3.0-ALPHA)

| Part | State |
|---|---|
| `core`, `config`, `app` | Done, tested |
| `plugins`, `sdk/rcpmod` | WASM module host and SDK done; sandbox tested against a hostile module |
| `modules/*` | Blackmagic Studio Camera 6K Pro: REST and event WebSocket, verified on hardware. `demo`: simulated. Other 14 models: controls and page declared, protocol not written |
| `surfaces/micropanel` | Driver done and verified on hardware |
| `surfaces/panel` | Panel controller wired in the app: displays, LEDs, camera select, keys, knobs, lever and ring with soft takeover. See [panel-mapping.md](panel-mapping.md) |
| `gui` | Native Fyne GUI, English and French |
| routers (Videohub, ATEM, vMix) | Not started |

See [app-api.md](app-api.md) for the API the GUI uses, [security.md](security.md) for the security
model and [Known vulnerabilities](vulnerabilities.html) for the vulnerabilities and their tests.
