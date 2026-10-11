# Panel mapping (ATEM Micro Camera Panel → app)

How the app drives the panel and what each physical control does. Code:
[internal/surfaces/panel](../internal/surfaces/panel). Protocol details live in
[atem-micro-camera-panel-protocol.md](atem-micro-camera-panel-protocol.md).

## Lifecycle

`panel.Controller` implements `app.Surface`; `cmd/rcp` registers it with
`app.Host.SetSurface`, and the Host runs it next to USB detection.

- Every second (1 s, backoff up to 10 s on errors) the controller tries to open and
  authenticate the panel. Re-authentication every 400 s is done by the driver.
- **ATEM Software Control running** (process `ATEM Software Control.exe`): the controller
  does not open the panel (both programs could open the HID device and fight over the
  displays). `PanelStatus` = connected, not active, detail
  "utilisé par un autre logiciel (fermez ATEM Software Control)". The check repeats every
  2 s while driving: if ATEM Software Control starts, the app releases the panel without
  blanking it.
- Open or authentication failure: detail "panneau détecté mais inaccessible, nouvel
  essai…", logged once per failure streak, retried with backoff.
- Unplug: the read fails, the session ends, `active` goes false and the controller retries.
- Shutdown (`Host.Stop`): digits and matrix are blanked and all 6 LED layers cleared, then
  the device is closed.
- Each session starts with all LED layers cleared (`micropanel.Open`, trap 7). Only LED
  bits 0–74 are ever set (trap 3; a test checks every frame).

`PanelStatus.connected` = plugged in, `PanelStatus.active` = opened and driven by the app.

## Output (app → panel)

Frames are rendered at ~30 Hz and only the reports that changed are sent.

| Panel element | Shows |
|---|---|
| Camera buttons 1–8 | empty slot: off · assigned: **blue** · active: **orange** (amber, GUI selection colour) · tally on: **red** (wins over orange). The LEDs are on/off per colour channel, so a dimmed "assigned" is not possible. |
| Camera matrix | Active slot number in orange, red when that camera's tally is on. `-` when no slot is selected. |
| ND (1 digit) | `nd` option index: 0 = clear, 1 = first filter… |
| GAIN (2 digits) | `gain` in dB: `00`, `06`, `-3`; `Lo`/`Hi` outside −9..99. Else `iso`: hundreds, read ×100 (`1.6` = 160, `16` = 1600), trailing point = thousands (`13.` = 12800). |
| SHUTTER (4 digits) | `shutterSpeed` without `1/` (`50`, `2000`), else `shutterAngle` in degrees (`180`, `172.8`). The first one whose value is known wins. |
| IRIS (3 digits) | `iris` in f-stops (`F2.8`, `F11`); a normalized iris (range 0..1) as a percentage. |
| MB (3 digits) | `masterBlack`; ranges with a step below 1 are shown ×100. |
| CC (1 digit) | Blank: no standard control yet. |
| Any display | **Blank** when the camera does not declare the control, **dashes** when the value is unknown (camera not connected; always the case while drivers are placeholders). Blank everywhere when the active slot is empty or no slot is selected. |
| Iris scale (LEDs 0–15) | Camera iris as a bar from the opening down to CLS (2.0, 2.8 … 16 on segments 1, 3 … 13). Off when unknown. While the lever waits for pickup, the segment under the **lever** blinks (2 Hz). |
| IRIS/MB ACTIVE | Red = lever and ring both drive the camera (picked up). White = at least one is waiting for pickup or its value is unknown. Off = camera has neither control. |
| ND / GAIN / SHUTTER ▲▼ | Lit when the value can still move that way (off when unknown). |
| BARS + BARS indicator | `colorBars` is on. |
| W/B, AUTO IRIS | Lit while the key is held (if the camera has the action). |
| PANEL ACTIVE (red) | Always on while the app drives the panel. |
| NETWORK | Active camera connected. |
| ALARM | Blinks while the active camera is in error state, and for 3 s after any command failed (`camera:commandFailed` or a failed panel write). A request for confirmation (operator lock) is not an error. |
| Others (CABLE, CALL, REL/ABS, EXT, scenes, STORE, PREVIEW) | Off. |

## Input (panel → app)

All Service calls run on one writer goroutine, so a slow camera never blocks the panel.

| Control | Action |
|---|---|
| CAMERA 1–8 | `SelectSlot(n)` for an assigned slot; the button of an empty slot is inert (LED off). Keys held when the panel connects are ignored. |
| ND ▲▼ | Next/previous `nd` option. |
| MASTER GAIN ▲▼ | `gain` ± its step (step 0 → range/20), else next/previous `iso` option. |
| SHUTTER ▲▼ | Next/previous `shutterSpeed` option, else `shutterAngle` through 11.25, 15, 22.5, 45, 72, 90, 144, 172.8, 180, 216, 270, 360. |
| AUTO IRIS | `autoIris` action. |
| W/B | `autoWhiteBalance` action. |
| BARS | Toggles `colorBars`, **only when its current value is known** (never guess what goes on air). |
| Iris lever | `iris` with soft takeover (below). Lever back = closed. f-stops: logarithmic from Max (closed) to Min (open), narrowed to the lens' own limits when the camera reports them (`core.Lens`), so the lever never asks for an aperture the lens lacks. Normalized iris: linear, 1 = open (TODO(verify) per driver). Quantized to the spec step. |
| Master black ring | `masterBlack`, linear over the range, soft takeover. |
| WHITE R/G/B knobs | `gainR/G/B`, relative. |
| BLACK R/G/B knobs | `liftR/G/B`, relative. |
| Other keys (SCENE 1–5, STORE, CC ▲▼, ON, BLACK/FLARE, IRIS/MB ACTIVE, CALL, PANEL ACTIVE, PREVIEW), top-right knob, SENS, COARSE | Not mapped; key presses are logged at debug. |

Rules common to every write:

- Nothing is written for a control the camera does not declare, a read-only control, or a
  control whose value is unknown (no stepping, no relative change, no lever).
- ▲▼ step from the value last known or last sent (optimistic), so fast presses accumulate.
- Continuous controls (lever, ring, knobs) are coalesced per control and sent at most
  ~30 times a second; the latest value is always the one sent last.
- Errors are logged once per (slot, control, error code). `NOT_IMPLEMENTED`,
  `NOT_CONNECTED`, `CONFIRM_REQUIRED` and `EMPTY_SLOT` go to debug. `CONFIRM_REQUIRED`
  (operator lock) is never retried or bypassed: the panel never calls
  `ConfirmSetControl`. A failed write triggers a re-read of the active camera's controls.

### Soft takeover (SPEC CB-1)

The lever and ring are absolute and cannot be moved by software. After a camera switch, a
reconnection, or a change of the value from elsewhere (GUI, camera, another event), the
lever only takes control once it **reaches** the camera value (within 1 % of travel) or
**crosses** it between two reports. Until then nothing is written. With an unknown value
nothing is ever written.

### Color knobs

The knobs report absolute positions 0..1024 that the panel keeps and clamps. Only the
change between two reports is used: one full travel (1024 counts) spans the control's whole
range (lift −2..2: 256 counts = 1.0). The value is clamped, not quantized, so slow turns
accumulate. Limitation: at the panel's end-stop (0 or 1024) the knob cannot go further in
that direction. Resyncing the knob position by writing feature 0x0E (untested) would fix it.
The knobs show ±1 count jitter at rest; it can cause a rare write of an unchanged value.

## Brightness

There is no brightness control known for this panel:

- ATEM Software Control has no panel brightness setting. Its only "brightness" strings
  belong to the chroma keyer.
- Read-only features: 0x04 = `60 10 41 00 00 f4 01` (meaning unknown), 0x06 = battery
  (`00 64`: not charging, 100 %), 0x15 = name "ATEM Micro Camera Panel". None is a brightness.
- LED layers 1–5 are the last candidate: `go run ./cmd/panelcheck -layers` lights camera
  button *n* white on layer *n−1* only (result pending an owner photo). If all six buttons
  look identical and steady, layers are not brightness and there is no hardware dimming.
  Software dimming (switching LEDs on and off quickly) would flicker over USB HID, so it is
  not offered.

## Hardware check

`go run ./cmd/panelcheck -t 60s` drives the panel through the controller with a fake rig
(cameras on slots 1, 2, 4; slot 2 active; slot 4 on air and in error). Input prints the Service
calls it would make. The panel is blanked on exit.
