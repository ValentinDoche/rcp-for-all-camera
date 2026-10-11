# RCP For All Camera

![Go](https://img.shields.io/badge/Go-1.27-00ADD8?logo=go&logoColor=white)
![Platform](https://img.shields.io/badge/platform-Windows-0078D6?logo=windows&logoColor=white)
![GUI](https://img.shields.io/badge/GUI-Fyne-4B8BBE)
![Status](https://img.shields.io/badge/status-work%20in%20progress-orange)

Turn a **Blackmagic ATEM Micro Camera Panel** (USB) into a universal camera control panel on
Windows, without an ATEM switcher. The panel and a native desktop app control Blackmagic,
Sony and Panasonic cameras over the network.

## Status

| Part | State |
|---|---|
| ATEM Micro Camera Panel (USB) | Working: buttons, knobs, iris lever, displays and LEDs |
| Desktop app (FR / EN, light / dark) | Working |
| Camera models and their parameters | Declared for 15 models |
| Camera network protocols | Not written yet: commands are not sent to cameras |
| Routers (Videohub, ATEM, vMix) | Not started |

Supported models: Blackmagic Pocket 4K, Pocket 6K G2, Pocket 6K Pro, Studio 4K Plus G2,
Studio 4K Pro G2, Studio 6K Pro; Sony FX3, FX6, FX9, FR7; Panasonic AW-UE100, AW-UE150,
AW-UE160.

## Build

Requires Go 1.27+ and a C compiler (MinGW-w64), because the GUI uses cgo.

```bash
winget install BrechtSanders.WinLibs.POSIX.UCRT
go build -ldflags "-H windowsgui" -o rcp.exe ./cmd/rcp
```

Close ATEM Software Control before starting the app: both cannot drive the panel at once.

## Tests

```bash
go test ./...
```

No hardware needed. `go run ./cmd/panelcheck -t 60s` shows a fake rig on a real panel.

## Documentation

- [Architecture](docs/architecture.md)
- [Adding a camera](docs/adding-a-camera.md)
- [Panel mapping](docs/panel-mapping.md)
- [Panel USB protocol](docs/atem-micro-camera-panel-protocol.md)
- [Translations](docs/i18n.md)
- [Security](docs/security.md)

## Disclaimer

Not affiliated with Blackmagic Design, Sony or Panasonic. Product names are trademarks of
their owners.
