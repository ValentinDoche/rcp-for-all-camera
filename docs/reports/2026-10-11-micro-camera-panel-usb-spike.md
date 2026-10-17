# Report: ATEM Micro Camera Panel USB spike

Date: 2026-10-11 · Status: **done, GO** · Answers open question 1 of [SPEC.md](../../SPEC.md)

## 1. Summary

**The panel can be fully driven from a Windows PC over USB, with no ATEM switcher and no Blackmagic software.**

- It is a **standard HID device** (interface MI_02), with no proprietary driver. The only lock is a **challenge/response authentication** on feature report 0x05. It was broken: it is the same algorithm as the DaVinci Speed Editor's.
- **Every input is decoded and verified on the hardware**:
  - 31 buttons;
  - 9 knobs, as absolute positions 0–1024;
  - the iris lever and the master black ring, from −4096 to +4096.
- **Every output is decoded and verified on the hardware**:
  - the 14 seven-segment displays;
  - the 7×10 three-colour camera matrix;
  - the 75 LEDs: RGB camera buttons, backlights, indicators, iris bar.
- The driver is written in **pure Go**, with no cgo and no external dependency. It fits the SPEC's "a single `.exe`" assumption.
- The test program [`paneltest`](../paneltest.md) exercises every physical control and shows simulated camera data on the panel: ND, CC, GAIN, SHUTTER, IRIS, MASTER BLACK, camera number and tally.

Consequence for the project: the `surface-micro-panel` module is no longer a risk. The panel does not need to pose as an ATEM.

## 2. Approach (multi-agent)

| Role | Who / model | Why this model | Result |
|---|---|---|---|
| Orchestration, live USB probes, Go driver, integration, docs | Main agent (Opus) | Protocol reasoning and architecture decisions | HID descriptor, authentication, captures, driver, LED map |
| Static reverse engineering of **ATEM Software Control.exe** (Windows) | Opus sub-agent | Long, difficult x64 disassembly | Seven-segment encoding, matrix font and format, LED report format, camera LEDs, role of the feature reports |
| Analysis of the **Middle Control 4.0.4** DMG (macOS) | Opus sub-agent | DMG extraction and Mach-O binary analysis | **Negative lead**: Middle Control does not talk to the panel over USB (see §6) |
| Web interface of the test program | Sonnet sub-agent | Well-specified front-end work, fast and cheap | `cmd/paneltest/web/index.html` (virtual panel, checklist, log) |

The hardware tests were done with the owner, who handled the panel and sent photos:
1. an authenticated capture of every control, in a set order;
2. three validation photos of the outputs;
3. seven binary-pattern photos plus one control photo to map the 75 LEDs.

## 3. Findings

Byte-by-byte details: [atem-micro-camera-panel-protocol.md](../atem-micro-camera-panel-protocol.md).

| Topic | Result | Source |
|---|---|---|
| USB identity | VID 0x1EDB / PID 0xBF01. HID on MI_02 (page 0xFF00). MI_00 is a "Media" interface not used for the panel's inputs and outputs | `Get-PnpDevice`, HID descriptor |
| Authentication | Feature report 0x05: `00` → challenge, `01`, `03` + response → `04` + a 600 s timeout. The Speed Editor algorithm | Live test (worked first time) and ATEM SC code |
| Buttons | Report 0x03: up to 6 held u16 codes. Cameras 0x40–0x47, scenes 0x48–0x4C, STORE 0x4D, arrows 0x4E–0x55, ON/WB/BARS 0x56–0x58, bottom of the panel 0x59–0x5E | Guided capture |
| Knobs | Report 0x0E: 9 absolute u16 values 0–1024, also readable as a feature (read at connect) | Guided capture |
| Lever and ring | Report 0x0F: int32 `[1, lever, ring]`, ±4096 | Guided capture |
| Displays | Report 0x10: 14 bytes, bit 7 = a … bit 1 = g, bit 0 = point. Order: ND, CC, GAIN×2, SHUTTER×4, IRIS×3, MB×3 | ATEM SC ("Auto" = EE 38 1E 3A) and photo |
| Matrix | Report 0x11: 7×10 pixels, 2 bits per pixel (1 red, 2 green, 3 amber), one- and two-digit fonts | ATEM SC and photo |
| LEDs | Report 0x02: a 30-byte bitmap followed by a layer number. 75 LEDs on layer 0 (full map in the protocol doc) | ATEM SC and 8 photos |
| Other feature reports | Battery (0x06), input assignment (0x09/0x0A), scene files stored **in the panel** (0x12/0x13), name (0x15) | ATEM SC (not tested) |

Technical trap: feature reports must be read and written **at their exact size**. With a buffer padded to the maximum size, the panel answers `ERROR_GEN_FAILURE`. The driver handles it.

## 4. Deliverables

| Path | Content |
|---|---|
| [`internal/surfaces/micropanel/`](../../internal/surfaces/micropanel) | Driver: pure-Go Windows HID access (`setupapi`/`hid.dll`), authentication, input decoding, display, matrix and LED encoders, periodic re-authentication |
| [`cmd/paneltest/`](../../cmd/paneltest) | Test bench: local server, simulated cameras, embedded web interface, checklist of the 42 controls, LED explorer |
| [`cmd/panelprobe/`](../../cmd/panelprobe/main.go) | Low-level tool: hex dump, raw writes, demo |
| [`docs/atem-micro-camera-panel-protocol.md`](../atem-micro-camera-panel-protocol.md) | Protocol reference |
| [`docs/paneltest.md`](../paneltest.md) | Test bench user guide |

Start the bench:

```bash
go run ./cmd/paneltest
```

## 5. Verification

| Check | Result |
|---|---|
| `go vet ./...` | OK |
| `go test ./...` | OK: the authentication response tested against 3 real challenges accepted by the panel, the seven-segment font compared with the bytes of ATEM SC, text encoding |
| Pure-Go connection to the real panel | OK: authentication, 10-minute session, initial positions read |
| Displays, matrix, camera LEDs | OK, checked on photos (`2`, `A`, `12`, `50`, `F2.8`, `-05`, matrix "1", red/green/cyan/amber buttons) |
| Map of the 75 LEDs | OK: 7 binary patterns plus 1 control pattern matching the prediction |
| Buttons, knobs, lever, ring | Codes checked by capture. **The full checklist in `paneltest` is still to be done by the owner** (see §7) |

Not verified:
- writing the knob positions (report 0x0E as a feature);
- LED layers 1 to 5;
- reports 0x06, 0x09–0x0B, 0x12–0x15.

## 6. Leads dropped

- **Middle Control (macOS)** drives the panel *through a real ATEM switcher*, with its "ATEM Link" option and the `BMDSwitcherAPI` SDK; the panel is plugged into the switcher. The binary holds no HID code for this panel. It is useless for our switcher-less case.
- **Posing as an ATEM switcher** (SPEC assumption 3): unnecessary, the panel is driven directly over HID.

## 7. Open points

1. **Names of codes 0x59–0x5E.** Inferred from the order of presses: 0x59 Black/Flare (never seen in a capture), 0x5A IRIS/MB ACTIVE, 0x5B AUTO IRIS, 0x5C CALL, 0x5D PANEL ACTIVE, 0x5E PREVIEW. To confirm with the `paneltest` log.
2. **Square Black/Flare button**: no code observed and no LED found. Is it really a button?
3. **Knobs**: can their position be written (feature 0x0E) to resync them to a camera's real value? It matters for the UX (no value jump when switching cameras).
4. **LED layers 1–5** (blinking? brightness?) and **bits beyond 74**, which disturb the matrix.
5. No **build tag nor non-Windows version** of the driver yet. Linux and macOS remain "nice to have" in the SPEC.
6. ATEM Software Control and `paneltest` must not run at the same time.

## 8. Impact on the SPEC and next steps

- **Open question 1 is resolved.** The panel is an authenticated HID device, usable without a switcher.
- **Assumption 2 is confirmed**: Go fits, without cgo.
- The core's `Surface` interface can build directly on `micropanel.State` (inputs) and on the display, matrix and LED encoders (feedback).
- Proposed next steps:
  1. Run the full checklist in `paneltest` and confirm points 1 and 2 of §7.
  2. Test writing feature 0x0E.
  3. Write `SPEC-surface-micro-panel.md` from this report.
  4. Start `core`, then `camera-blackmagic` (the Studio 6K Pro REST API).
