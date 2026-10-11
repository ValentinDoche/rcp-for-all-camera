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
| GAIN (2 digits) | **W/B mode**: `tint`, see [White balance mode](#white-balance-mode-wb). Otherwise `gain` in dB: `00`, `06`, `-3`; `Lo`/`Hi` outside −9..99. Else `iso`: hundreds, read ×100 (`1.6` = 160, `16` = 1600), trailing point = thousands (`13.` = 12800). |
| SHUTTER (4 digits) | **W/B mode**: `whiteBalance` in Kelvin (`5600`; from 10000 K in thousands, `10.0`, `12.5`). Otherwise `shutterSpeed` without `1/` (`50`, `2000`), else `shutterAngle` in degrees (`180`, `172.8`). The first one whose value is known wins. **While COARSE is turned** (and 5 s after its last movement): the iris window centre as an f-stop (`F5.6`, `F11`, `F22`), blinking at 2 Hz, with the SHUTTER ▲▼ LEDs off (see [Iris lever window](#iris-lever-window-coarse-and-sens)). |
| IRIS (3 digits) | `iris` in f-stops (`F2.8`, `F11`); a normalized iris (range 0..1) as a percentage. |
| MB (3 digits) | `masterBlack`, signed: `+12`, `-5`, `0` (no sign at zero). Ranges with a step below 1 are shown ×100 (`+12` = 0.12). Beyond ±99 (set from the GUI or the camera, the ring cannot go there) it shows `+Hi` / `-Lo`. The `+` is drawn with segments e, f, g (left half of a cross; TODO(verify) on the panel). |
| CC (1 digit) | Blank: no standard control yet. |
| Any display | **Blank** when the camera does not declare the control, **dashes** when the value is unknown (camera not connected; always the case while drivers are placeholders). Blank everywhere when the active slot is empty or no slot is selected. |
| Iris scale (LEDs 0–15) | Camera iris as a bar from the opening down to CLS (2.0, 2.8 … 16 on segments 1, 3 … 13), **masked to the lever window**: segments the lever cannot reach are always off (CLS too when the window does not reach the closed end). Off when unknown. While the lever waits for pickup, the segment under the **lever** blinks (2 Hz). **While SENS is turned** (and 5 s after): the window itself, every reachable segment lit. |
| IRIS/MB ACTIVE | Red = lever and ring both drive the camera (picked up). White = at least one is waiting for pickup or its value is unknown, or panel control is off. Off = camera has neither control. |
| ND / GAIN / SHUTTER ▲▼ | Lit when the value can still move that way (off when unknown). In W/B mode GAIN ▲▼ follow `tint` and SHUTTER ▲▼ `whiteBalance`. SHUTTER ▲▼ are off during the COARSE preview. |
| BARS + BARS indicator | `colorBars` is on. |
| W/B | Lit while the white balance mode is on. **While W/B is held** on a camera with `autoWhiteBalance`: blinks (2 Hz), then steady once the auto white balance is triggered (after 1 s), until released. |
| AUTO IRIS | Lit while the key is held (if the camera has the action). |
| PANEL ACTIVE | **Red** while the panel controls the cameras (default at start), **white only** when turned off with the key (see [Panel control off](#panel-control-off-panel-active)). |
| NETWORK | Active camera connected. |
| ALARM | Blinks while the active camera is in error state, and for 3 s after any command failed (`camera:commandFailed` or a failed panel write). A request for confirmation (operator lock) is not an error. |
| STORE | **Red** while Store is armed (shared with the GUI's Store button), off at rest (the white LED is never used). |
| SCENE 1–5 | Lit for the presets stored on the active slot (`Rig().Slots[i].Presets`). While Store is armed, all five **blink** (2 Hz) to invite a choice. Off with no active camera. |
| Others (CABLE, CALL, REL/ABS, EXT, SCENE FILE, PREVIEW) | Off. |

## Input (panel → app)

All Service calls run on one writer goroutine, so a slow camera never blocks the panel.

| Control | Action |
|---|---|
| CAMERA 1–8 | `SelectSlot(n)` for an assigned slot; the button of an empty slot is inert (LED off). Keys held when the panel connects are ignored. |
| ND ▲▼ | Next/previous `nd` option. |
| MASTER GAIN ▲▼ | `gain` ± its step (step 0 → range/20), else next/previous `iso` option. **W/B mode**: `tint` ± its step. |
| SHUTTER ▲▼ | Next/previous `shutterSpeed` option, else `shutterAngle` through 11.25, 15, 22.5, 45, 72, 90, 144, 172.8, 180, 216, 270, 360. **W/B mode**: `whiteBalance` ± max(its step, 100 K). |
| AUTO IRIS | `autoIris` action. |
| W/B | **Short press** (released within 1 s): toggles the **white balance mode** (below). **Long press** (held 1 s): runs `autoWhiteBalance` once, and the release then does not toggle the mode. A camera without `autoWhiteBalance`: the long press does nothing. |
| PANEL ACTIVE | Turns panel control off and on (below). |
| STORE | Toggles the app's shared Store arming: `SetStoreArmed(!StoreArmed())`. Works with or without an active camera. |
| SCENE 1–5 | Store armed: `StorePreset(active slot, n)` (the app disarms). Otherwise `RecallPreset(active slot, n)` if preset *n* is stored on the active slot, else nothing. No active camera: nothing. See [Presets](#presets-store-and-scene-15). |
| BARS | Toggles `colorBars`, **only when its current value is known** (never guess what goes on air). |
| Iris lever | `iris` with soft takeover (below). Lever back = closed. f-stops: logarithmic from Max (closed) to Min (open), narrowed to the lens' own limits when the camera reports them (`core.Lens`), so the lever never asks for an aperture the lens lacks, then to the **lever window** set by COARSE and SENS (whole range by default). Normalized iris: linear over the whole range, 1 = open (TODO(verify) per driver). Quantized to the spec step. |
| Master black ring | `masterBlack`, linear, soft takeover. The travel is limited to what the MB display shows as a number: −0.99..+0.99 for ranges shown ×100 (Blackmagic −2..2), −99..+99 for integer ranges, intersected with the range. A value set beyond that elsewhere cannot be picked up by the ring until it comes back inside. |
| WHITE R/G/B knobs | `gainR/G/B`, relative. |
| BLACK R/G/B knobs | `liftR/G/B`, relative. |
| COARSE, SENS | Set the iris lever window (below). They never write to the camera. |
| Other keys (CC ▲▼, ON, BLACK/FLARE, IRIS/MB ACTIVE, CALL, PREVIEW), top-right knob | Not mapped; key presses are logged at debug. |

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

### Panel control off (PANEL ACTIVE)

PANEL ACTIVE toggles whether the panel controls the cameras. It is **on** at start and
not persisted (each app start is on).

- **Off**: every input is ignored except PANEL ACTIVE itself: keys (CAMERA, ▲▼, W/B,
  STORE, SCENE, BARS, AUTO IRIS…), color knobs, iris lever, master black ring, COARSE and
  SENS. Nothing is written. The panel still renders the active camera and follows the
  app (camera selected in the GUI, tally, Store arming). PANEL ACTIVE is white, IRIS/MB
  ACTIVE white, the lever pickup hint does not blink. Turning off leaves the W/B mode.
- **Back on**: nothing jumps. The color knobs count from their current position, the
  lever and ring wait for pickup again (soft takeover), and the iris lever window is taken
  from where COARSE and SENS are now (no preview, nothing written).

### Presets (STORE and SCENE 1–5)

The app keeps 5 presets per slot and one **Store arming** shared by the panel and the GUI
(`app.Service.SetStoreArmed`, event `preset:storeArmed`). The panel keeps a local copy,
read with `StoreArmed()` when it starts and then updated from the event, so pressing Store
in the GUI lights the panel's STORE and vice versa.

- STORE toggles the arming. The panel never sets the LED itself: it follows the event.
- SCENE *n* is resolved on the writer goroutine, after any earlier STORE press was
  applied: armed → store preset *n* (overwriting it); not armed → recall it if it exists.
  After a scene key the active camera's controls are re-read, so the displays follow a
  recall and the lever/ring pick up the recalled values again (soft takeover).
- Store, recall and arming errors light ALARM like any failed command.

### White balance mode (W/B)

A short press on W/B (released within 1 s) toggles a mode where the SHUTTER and MASTER
GAIN keys and displays set the white balance instead of shutter and gain, e.g. `5600`
and `+3`. The W/B LED is lit while the mode is on. Holding W/B for 1 s runs the auto
white balance instead (the LED blinks while held, then stays on once triggered); a
camera without auto white balance ignores the long press.

- SHUTTER ▲▼: `whiteBalance` ± one step of at least **100 K** (max(spec step, 100 K)),
  quantized to the spec step. The SHUTTER display shows the Kelvin value: `5600`, and
  from 10000 K in thousands with one decimal (`10.0` = 10000 K, `12.5` = 12500 K).
- MASTER GAIN ▲▼: `tint` ± its step. The GAIN display shows the tint on 2 digits:

  | Tint | Display |
  |---|---|
  | 0 | `0` |
  | +1..+9 | `+1`..`+9` (`+` = left half of a cross, as on MB) |
  | −1..−9 | `-1`..`-9` |
  | +10..+99 | `10`..`99` (no sign: no room) |
  | −10..−99 | the two digits with **both decimal points lit** = minus (`1.2.` = −12, `5.0.` = −50) |
  | beyond ±99 | `Hi` / `Lo` |

- The ▲▼ LEDs show whether `whiteBalance` / `tint` can still move (off when unknown or
  not declared). A display is blank when the camera lacks that control (e.g. no tint on
  Panasonic), dashes when the value is unknown.
- The mode cannot be entered when the camera declares neither `whiteBalance` nor `tint`
  (W/B then does nothing). A camera switch leaves the mode; so does a camera that no
  longer declares either control.
- The COARSE preview keeps priority on the SHUTTER display (and switches SHUTTER ▲▼ off)
  while it lasts, in the mode too; GAIN keeps showing the tint.

TODO(verify) on hardware: readability of `+` and of the double decimal point for negative
tints.

### Iris lever window (COARSE and SENS)

Broadcast RCP convention: COARSE sets **where** the lever works, SENS **how wide**.
Only for an iris in f-stops; a normalized iris always uses its whole range.

- **COARSE** (knob index 8) sets the window centre C, logarithmic in f-number from the
  iris Min (narrowed to the lens) with the knob fully counter-clockwise (0 = most open)
  to Max fully clockwise (1024 = most closed).
- **SENS** (knob index 7) sets the window width W in stops, linear from **1 stop** fully
  counter-clockwise (0) to the **whole range** fully clockwise (1024 = the behaviour
  without the knobs).
- Window = C / 2^(W/4) .. C × 2^(W/4) in f-number (one stop = ×√2). When it passes an end
  of the range it is shifted to keep its width, then clamped.
- The lever maps logarithmically over the window (back = its closed end, forward = its
  open end). Turning COARSE or SENS writes nothing to the camera: it releases the iris
  pickup, so the lever must reach the camera iris again in the new window. **A camera
  iris outside the window cannot be picked up** until the window is moved over it.
- The knob positions are read at connect (feature 0x0E) and on every change, with the
  same ±1 count deadband as the color knobs. Until they have been read the window is the
  whole range. Knobs found at connect start no preview.
- Previews last **5 s** after the knob's last movement: COARSE blinks the centre on the
  SHUTTER display, SENS shows the window on the iris scale.

TODO(verify) on hardware: knob direction (0 = fully counter-clockwise is assumed for both)
and whether the knobs reach 0 and 1024 at their end-stops.

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
