# RCP For All Camera

[![CI](https://github.com/ValentinDoche/rcp-for-all-camera/actions/workflows/ci.yml/badge.svg)](https://github.com/ValentinDoche/rcp-for-all-camera/actions/workflows/ci.yml)
![Go](https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white)
![Platform](https://img.shields.io/badge/platform-Windows-0078D6?logo=windows&logoColor=white)
![GUI](https://img.shields.io/badge/GUI-Fyne-4B8BBE)
![Version](https://img.shields.io/badge/version-0.3.0--ALPHA-orange)

Turn a **Blackmagic ATEM Micro Camera Panel** (USB) into a universal camera control panel on
Windows, without an ATEM switcher. The panel and a native desktop app control Blackmagic,
Sony and Panasonic cameras over the network. Each camera model is a **module**: one
WebAssembly file in the `modules` folder, running in a sandbox.

## Status (0.3.0-ALPHA)

| Part | State |
|---|---|
| ATEM Micro Camera Panel (USB) | Working: buttons, knobs, iris lever, displays and lights; on-screen simulator |
| Desktop app (English / French, light / dark) | Working |
| Camera modules | Blackmagic Studio Camera 6K Pro: working over the network. Demo camera: simulated. 14 other models: controls declared, protocol not written yet |
| Routers (Videohub, ATEM, vMix) | Not started |

Declared models: Blackmagic Pocket 4K, Pocket 6K G2, Pocket 6K Pro, Studio 4K Plus G2,
Studio 4K Pro G2, Studio 6K Pro; Sony FX3, FX6, FX9, FR7; Panasonic AW-UE100, AW-UE150,
AW-UE160; generic Sony and Panasonic.

## Build

Requires Go 1.27+ and a C compiler (MinGW-w64), because the GUI uses cgo.

```bash
winget install BrechtSanders.WinLibs.POSIX.UCRT
go build -ldflags "-H windowsgui" -o dist/rcp.exe ./cmd/rcp
go run ./cmd/buildmodules
```

The modules land in `dist/modules`, next to `dist/rcp.exe`. Start `dist/rcp.exe`
(`-sim` for the on-screen panel). Close ATEM Software Control first: both cannot drive the
panel at once.

The exe icon comes from `cmd/rcp/rsrc_windows_amd64.syso`, built from the app logo. After a
logo change, run `go generate ./cmd/rcp` (needs `windres`, shipped with MinGW-w64).

## Tests

```bash
go test ./...
```

No hardware and no network needed. `go run ./cmd/panelcheck -t 60s` shows a fake rig on a
real panel.

## Documentation

The documentation is HTML; serve the `docs` folder (`python -m http.server --directory docs`)
and open:

- [Documentation home](docs/index.html): what the app does, quick start, architecture, panel,
  window, data and security, build and tests
- [Camera modules](docs/modules-camera.html): write a module for a new camera
- [Known vulnerabilities](docs/vulnerabilities.html): every known vulnerability and the test
  that proves it is fixed

Reference pages: [architecture](docs/architecture.md), [adding a camera](docs/adding-a-camera.md),
[panel mapping](docs/panel-mapping.md), [panel USB protocol](docs/atem-micro-camera-panel-protocol.md),
[translations](docs/i18n.md), [security model](docs/security.md),
[Go engineering rules](docs/go-engineering-rules.md).

## Disclaimer

Not affiliated with Blackmagic Design, Sony or Panasonic. Product names are trademarks of
their owners.
