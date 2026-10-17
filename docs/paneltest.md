# paneltest — ATEM Micro Camera Panel test bench

A small Windows program that connects to the ATEM Micro Camera Panel over USB, with
no ATEM switcher and no Blackmagic software running. It lets you check every
physical control and drives the panel's displays with **simulated camera data**.

## Run

```bash
go run ./cmd/paneltest
```

or build a single exe:

```bash
go build -o paneltest.exe ./cmd/paneltest
```

It opens <http://127.0.0.1:8765> in the browser. Use `-addr` to change the address
and `-no-browser` to skip opening it. No driver install, no admin rights, no cgo.

> Close **ATEM Software Control** first. It also authenticates the panel and
> writes to its displays, so the two programs would fight over it.

## What the page shows

- **Virtual panel.** Every button lights up while held and gets a green outline
  once it has been tested. Knobs show their position from 0 to 1024. The lever
  and ring show their position from −4096 to 4096.
- **Checklist.** All 42 physical controls (31 buttons, 9 knobs, the lever and the
  ring), with progress shown as X / 42. *Reset checklist* starts over.
- **Log.** Every press, release and movement, with timestamps.
- **Simulated data.** The state of the fake camera that the panel controls.
- **Connection.** Connection status, USB serial number and the raw report 0x04. The
  program reconnects on its own if the panel is unplugged.

## Simulated camera behaviour

| Panel control | Effect on the fake camera, and what the panel shows |
|---|---|
| Camera 1–8 | Selects the camera: its button turns **red**, the matrix shows its number in red, and all displays switch to that camera's values |
| PREVIEW held + camera N | Sets the preview camera (button turns **green**) |
| ND ▲▼ | ND step 0–3 (CLEAR, 1/4, 1/16, 1/64), on the **ND** display |
| CC ▲▼ | CC filter A, b, C, d, E, on the **CC** display |
| MASTER GAIN ▲▼ | −6 to +36 dB in 3 dB steps, on the **MASTER GAIN** display |
| SHUTTER ▲▼ | 1/24 to 1/2000, on the **SHUTTER** display |
| Iris lever | f/22 to f/1.8 on a log scale, on the **IRIS** display (`F2.8`) |
| Master black ring | −99 to +99, on the **MASTER BLACK** display |
| White / Black R G B knobs | Stored as the white/black balance values, shown in the web page |
| AUTO IRIS | Toggles auto iris; the IRIS display shows `Aut` |
| W/B | Cycles the white balance between 3200 K, 4300 K and 5600 K |
| ON, BARS | Toggle flags |
| STORE then SCENE n | "Stores" scene n (logged only); SCENE n alone "recalls" it |
| PANEL ACTIVE | Toggles panel active: button red when on, white when off; the matrix turns amber when off |
| IRIS/MB ACTIVE | Enables the lever and ring (red) or freezes them (white) |
| CALL / PREVIEW | Lit while held |

LEDs driven from the simulated state:
- ▲/▼ arrows are lit while the value can still move in that direction.
- The iris scale lights from the current f-stop down to CLS.
- SCENE n stays lit once a scene has been stored for the selected camera. STORE is red while armed, together with the SCENE FILE indicator.
- ON lights REL (ABS when off). BARS lights its button and the BARS indicator. W/B flashes for 1 s.
- EXT lights from 12 dB of gain, D EXT from 24 dB and ALARM from 30 dB.
- NETWORK and CABLE are lit while the panel is connected.

## Test buttons

- **Test afficheurs.** Shows `8.`, then `0`–`9`, `A`–`F` and `-` on every digit.
- **Test matrice.** Counts 1–12 on the matrix in red, green and amber, then cycles
  the camera buttons through red, green, blue, amber, cyan, magenta and white.
- **Test LEDs (explorateur).** Lights a single LED index, shown on the SHUTTER
  display, with the layer shown on ND. **SHUTTER ▲▼** steps through the LEDs and
  **ND ▲▼** changes the layer. Useful to check the LED map in the
  [protocol doc](atem-micro-camera-panel-protocol.md#53-leds--report-0x02) or to study layers 1–5.
  Click the button again to leave.

## Code layout

| Path | Role |
|---|---|
| `internal/surfaces/micropanel/hid_windows.go` | Pure-Go HID access through `setupapi`, `hid.dll` and `kernel32` |
| `internal/surfaces/micropanel/auth.go` | Challenge/response |
| `internal/surfaces/micropanel/protocol.go` | Report IDs, key codes, input decoding |
| `internal/surfaces/micropanel/display.go`, `matrix.go`, `leds.go` | Output encoders |
| `internal/surfaces/micropanel/panel_windows.go` | `Panel`: open, authenticate, read loop, re-auth, writes |
| `cmd/paneltest/` | This tool (server, fake camera, embedded web page in `web/index.html`) |
| `cmd/panelprobe/` | Raw protocol probe (hex dump, raw writes) |
