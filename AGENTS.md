# AGENTS.md — context for any agent working on this repo

Read this first. It says what the project is, what has been done, where everything
is, and the traps already hit. Last updated: 2026-10-11.

## 1. Project in one paragraph

**RCP For All Camera**: make a Blackmagic **ATEM Micro Camera Panel** (USB) work as a
universal camera control panel on a Windows PC **with no ATEM switcher**. It will
control Blackmagic (Pocket 4K/6K, Studio 4K/6K), Sony (FX3, FX6, FX9, FR7) and
Panasonic (AW-UE100/150/160) cameras over the network. Camera buttons 1–8 also route the source to an aux on a Videohub,
an ATEM or vMix. The code is open source and modular (surface / camera / router
modules). Language: **Go**, built as a single `.exe`, with a **native Fyne GUI**
(the owner refused a web UI: no Wails, no HTML/WebView).
Owner: Valentin (writes in French, so answer in French).

- Intent: [docs/intent/rcp-for-all-camera.md](docs/intent/rcp-for-all-camera.md)
- Spec, capability map and build order: [SPEC.md](SPEC.md) (still DRAFT; open question 1 is resolved)

## 2. Status

| Module (SPEC id) | State |
|---|---|
| `surface-micro-panel` | **Driver done and verified on hardware** (Windows only): [internal/surfaces/micropanel](internal/surfaces/micropanel) |
| panel controller | **Wired in the app**: [internal/surfaces/panel](internal/surfaces/panel), mapping in [docs/panel-mapping.md](docs/panel-mapping.md). Hardware render check pending owner photo (`go run ./cmd/panelcheck -t 60s`) |
| `core`, `config`, `app` | **Done, unit tested**: contracts, registry, rig persistence, settings (bind NIC, autostart, operator lock, logs, export/import), camera event forwarding, `app.Service` API ([docs/app-api.md](docs/app-api.md)) |
| cameras | **Models + control specs declared, no protocol** (`camerakit.Placeholder` returns `NOT_IMPLEMENTED`). 15 models; Panasonic UE150/UE160 complete from SKAARHOJ lists |
| `gui` | Native Fyne GUI in [internal/gui](internal/gui), FR/EN via [internal/i18n](internal/i18n) ([docs/i18n.md](docs/i18n.md)), entry point [cmd/rcp](cmd/rcp) |
| routers | **Not started** |

Architecture overview: [docs/architecture.md](docs/architecture.md). Security model: [docs/security.md](docs/security.md).

Phase 1 (USB feasibility spike) is complete, verdict **GO**. Full report:
[docs/reports/2026-10-11-micro-camera-panel-usb-spike.md](docs/reports/2026-10-11-micro-camera-panel-usb-spike.md) (French).

## 3. Repo map

```
AGENTS.md                         ← this file
SPEC.md                           ← project spec (assumptions, modules, boundaries)
go.mod                            ← module github.com/valentindoche/rcp-for-all-camera, dep: fyne
cmd/rcp/                          ← the app (GUI) entry point
internal/core/                    ← contracts: ControlSpec/Value, Camera/Driver, ModelInfo, Registry, errors
internal/config/                  ← rig.json store (atomic, 0600) + Secrets (passwords never in rig.json)
internal/app/                     ← Service (API used by the GUI) + Host (lifecycle, panel polling)
internal/cameras/                 ← Register(); blackmagic/ sony/ panasonic/ drivers;
                                    camerakit (Placeholder), speckit (spec builders), cameratest (contract suite)
internal/gui/                     ← Fyne GUI (themes clair/sombre)
docs/architecture.md, docs/app-api.md, docs/adding-a-camera.md, docs/security.md
internal/surfaces/micropanel/
  hid_windows.go                  ← pure-Go HID: setupapi + hid.dll + kernel32 (no cgo)
  auth.go, auth_test.go           ← challenge/response (feature report 0x05)
  protocol.go                     ← report IDs, key codes, State, input parsing
  panel_windows.go                ← Panel: Open/authenticate/Run (read loop + re-auth)/writes
  display.go, display_test.go     ← 7-segment encoder (report 0x10)
  matrix.go                       ← camera dot-matrix encoder + fonts (report 0x11)
  leds.go                         ← LED bitmap + full LED index constants (report 0x02)
cmd/paneltest/                    ← test bench: HTTP server + SSE + fake cameras
  main.go, camera.go
  web/index.html                  ← embedded UI (French), go:embed
cmd/panelprobe/main.go            ← low-level probe: hex dump, raw -out/-feat/-get, -demo
docs/atem-micro-camera-panel-protocol.md   ← THE protocol reference (byte layouts, maps)
docs/paneltest.md                 ← how to use the test bench
docs/reports/2026-10-11-micro-camera-panel-usb-spike.md ← spike report (FR)
Middle-Control-MacOS-4.0.4.dmg    ← third-party app; analysed, useless for USB (see §7)
paneltest.exe                     ← local build output, rebuild instead of trusting it
```

Git repo on `main`, Conventional Commits (`feat(core): …`, `fix(gui): …`, `refactor`,
`docs`, `test`, `chore`), each commit ends with the Co-Authored-By line.

## 4. Commands

Fyne needs cgo. gcc (WinLibs MinGW-w64, installed with winget) is on the user PATH, but
an already-open shell may not see it:

```bash
export PATH="/c/Users/VALENTIN/AppData/Local/Microsoft/WinGet/Packages/BrechtSanders.WinLibs.POSIX.UCRT_Microsoft.Winget.Source_8wekyb3d8bbwe/mingw64/bin:$PATH" CGO_ENABLED=1
go build -ldflags "-H windowsgui" -o rcp.exe ./cmd/rcp   # the app (first build ~4 min)
go test -race ./internal/...
```

```bash
go vet ./...
go test ./...                       # unit tests, no hardware needed
go run ./cmd/paneltest              # test bench → http://127.0.0.1:8765 (-no-browser, -addr)
go build -o paneltest.exe ./cmd/paneltest
go run ./cmd/panelprobe -t 10s      # authenticate + dump input reports as hex
go run ./cmd/panelprobe -demo -t 0  # sample values on displays/matrix/camera LEDs
go run ./cmd/panelprobe -out 10:<28 hex chars> -get 4:8 -t 0
```

`paneltest` also has raw endpoints for experiments: `POST /api/raw?out=<id>:<hex>`
pauses rendering for 1 h, and `POST /api/resume` gives rendering back.
`/api/test/{displays|matrix|leds}` runs the test patterns; `leds` toggles the LED
explorer (SHUTTER ▲▼ = LED index, ND ▲▼ = layer).

## 5. Protocol cheat-sheet (details in docs/atem-micro-camera-panel-protocol.md)

- USB VID `0x1EDB`, PID `0xBF01`. HID interface **MI_02**, usage page `0xFF00`. MI_00 is a "Media" interface and is not used.
- **Auth required** before any input arrives: feature `0x05` gives the challenge →
  `05 01` → `05 03` + response, and the panel replies `04` + a 600 s timeout. The
  algorithm is the DaVinci Speed Editor one. Re-auth every 400 s.
- **Inputs**
  - `0x03`: up to 6 held key codes (u16). Cameras 0x40–47, scenes 0x48–4C, STORE 4D,
    ND▲▼ 4E/4F, CC 50/51, GAIN 52/53, SHUTTER 54/55, ON 56, W/B 57, BARS 58, bottom buttons 59–5E (names inferred).
  - `0x0E`: 9 × u16 absolute knob positions 0..1024. Order: WR, BR, WG, BG, WB, BB, top-right knob, SENS, COARSE.
  - `0x0F`: int32 `[1, iris lever, master-black ring]`, ±4096.
  - 0x0E and 0x0F can also be read as feature reports to get the current positions.
- **Outputs**
  - `0x10`: 14 seven-segment digits. Bit 7 = a … bit 1 = g, bit 0 = DP. Order: ND, CC, GAIN×2, SHUTTER×4, IRIS×3, MB×3.
  - `0x11`: matrix 7×10, 2 bits per pixel (1 red, 2 green, 3 amber), p = row*10+col, LSB first.
  - `0x02`: 30-byte LED bitmap + layer byte. Everything is on layer 0, LEDs 0–74. Iris bar 0–15,
    camera RGB 16+3c..18+3c, scene/store 40–47, arrows 48–55, REL/ON/ABS/WB/BARS 56–61,
    D EXT/EXT 62/63, IRIS/MB 64(w)/65(r), AUTO IRIS 66, CALL 67(w)/68(r),
    NETWORK/ALARM/CABLE 69–71, PANEL ACTIVE 72(w)/73(r), PREVIEW 74.

## 6. Traps already hit (don't repeat them)

1. **Feature reports need their exact size.** Padding a `HidD_GetFeature`/`SetFeature`
   buffer to the 64-byte max makes the panel fail with `ERROR_GEN_FAILURE`.
   Output reports (`WriteFile`) are fine padded to 33 bytes.
2. Use **two handles** on the HID path (one blocking reader, one for writes/features),
   because synchronous Windows handles serialize I/O.
3. **Do not set LED bits above 74**: they corrupt the camera matrix.
4. **ATEM Software Control must be closed**: it also authenticates the panel and overwrites the displays.
5. Python on this machine defaults to **cp1252**. Always `open(..., encoding='utf-8', newline='\n')`
   when editing files by script. A cp1252 write once truncated `cmd/paneltest/main.go`.
6. Scripted edits produced CRLF, so run `gofmt -w .` after them.
7. **LED layers are persistent in the panel.** A bit left set on layers 1–5 (for
   example by the LED explorer) stays lit forever, even though the program keeps
   rewriting layer 0. `micropanel.Open` clears all 6 layers, and `Panel.ClearLEDs(keep)`
   wipes the others. Bug seen: the iris "11" segment stuck on.
8. Input reports are only sent on change, so read features 0x0E/0x0F at connect
   (already done in `Panel.Run`).
9. The **Bash tool mangles some heredocs** containing apostrophes; write files with the
   Write tool instead.
10. The owner wants a **native** GUI. Wails (HTML in WebView2) was built then removed.

## 7. What was tried and abandoned

- **Middle Control 4.0.4 (macOS DMG)** has no USB code for this panel. It goes through a real
  ATEM switcher ("ATEM Link" + `BMDSwitcherAPI`). Don't re-analyse it.
- **Posing as an ATEM switcher** is unnecessary because the panel is driven directly over HID.
- The protocol was recovered from live USB captures and static analysis of
  `C:\Program Files (x86)\Blackmagic Design\Blackmagic ATEM Switchers\ATEM Software Control\ATEM Software Control.exe`.
  Panel code addresses (image base 0x140000000) are listed in the spike report.
  Temporary analysis files lived in the session scratchpad and are **not** kept.

## 8. Open items (pick up here)

1. Confirm the names of key codes 0x59–0x5E in the `paneltest` log, and check whether the
   square BLACK/FLARE button sends anything (no code and no LED found so far).
2. Run the full 42/42 checklist in `paneltest` with the owner.
3. Test **writing** feature 0x0E (knob positions), so knobs can resync to a camera's real value when switching cameras.
4. Meaning of LED layers 1–5 and of LED bits above 74.
5. Untested, from static analysis only:
   - feature 0x06 (battery)
   - feature 0x09/0x0A (input IDs)
   - feature 0x12/0x13 (scene files stored in the panel)
   - feature 0x15 (name)
   - feature 0x04
   - input 0x0D
6. No non-Windows build of the driver yet (`*_windows.go` only). Keep the HID layer behind the `Surface` interface.
7. **Owner requirement CB-1 (see SPEC.md "Control Behaviour Requirements")**: iris lever pickup / soft takeover. Implemented in internal/surfaces/panel (to verify on hardware).
   After a camera switch, the lever only takes control once it reaches the camera's
   current iris (camera at F10, lever at F4 → nothing until the lever reaches F10).
   The same applies to the master-black ring. `paneltest` (old bench) still jumps the iris.
8. Next: confirm the panel rendering with owner photos (`cmd/panelcheck`, and
   `-layers` for the brightness question: no hardware brightness found so far), then the
   first real protocol (`camera-blackmagic` REST, or Panasonic AW CGI), then routers. Every control range marked `TODO(verify)` in
   `internal/cameras/*/specs.go` must be checked against official docs or hardware.

## 9. Working rules (from SPEC.md boundaries, plus the owner's preferences)

- Idiomatic Go, `gofmt`, errors wrapped with `%w`, `context.Context` on I/O. **No new runtime dependency without asking.**
- Keep protocol details inside the driver module. Log device commands at debug level. Survive unplug and reconnect.
- **Ask first** before sending anything that could change recording or output on live gear.
- Hardware verification works best when the owner sends phone photos. Design
  experiments that need few photos, for example one binary-coded pattern per photo
  with a pattern label shown on the SHUTTER display.
- The owner asked for multi-agent work with the model chosen per task: Opus for
  reverse-engineering and architecture, Sonnet for well-specified UI/boilerplate.
