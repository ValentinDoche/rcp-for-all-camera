# Panel simulator

A software ATEM Micro Camera Panel, to work on the app without the hardware.
Code: [internal/surfaces/panelsim](../internal/surfaces/panelsim).

```bash
go build -ldflags "-H windowsgui" -o rcp.exe ./cmd/rcp
./rcp.exe -sim
```

With `-sim` the app opens a second window, "Simulateur ATEM Micro Camera Panel", and
the panel controller ([internal/surfaces/panel](../internal/surfaces/panel)) drives it
instead of the USB panel. The USB panel is ignored while `-sim` is on.

## How it works

`panelsim.Sim` implements `panel.Device`, the interface the controller already uses for
`*micropanel.Panel`. The controller is unchanged: it opens the sim, writes the same
digit, matrix and LED reports, reads the same input states. What is tested on the
simulator is the real rendering and input mapping of
[panel-mapping.md](panel-mapping.md), not a copy of it.

The sim follows the hardware where the controller depends on it:

- knob counters are absolute and clamp at 0 and 1024 (the knob needle stops at ±150°);
- feature 0x0E and 0x0F give the current positions; 0x0E can be written (knob recenter)
  and a padded feature size fails (trap 1);
- LED layers persist and are ORed; `Open` clears them like `micropanel.Open`;
- the iris lever and master black ring stay where they are left (soft takeover can be
  tried: move the lever away, switch camera, come back).

Not simulated: authentication, re-authentication, ATEM Software Control taking the
panel, and the unknown parts of the protocol (LED layers 1–5 meaning, features 0x04,
0x06, 0x09/0x0A, 0x12/0x13, 0x15).

## Mouse

| Panel control | Mouse |
|---|---|
| Any key | Click: held while the button is down (W/B long press, AUTO IRIS held work) |
| Knobs (WHITE/BLACK R G B, top-right, SENS, COARSE) | Drag up/down (up = clockwise, 256 px for the whole travel) or the mouse wheel (8 counts per notch) |
| Iris lever | Drag the lever area up (forward = open) or down (back = closed) |
| Master black ring | Drag the lever area left/right, or the mouse wheel over it |
| "Panneau branché" | Unplugs and replugs the panel (the session ends like a USB unplug) |

Closing the simulator window unplugs the panel; the main window keeps running. The
lever and ring positions, as the panel would report them (±4096), are shown under the
lever.

The layout follows the product drawing. The key codes and LED indexes come from
[atem-micro-camera-panel-protocol.md](atem-micro-camera-panel-protocol.md), so the
bottom-row names (0x59–0x5E) carry the same uncertainty as there.

`PANELSIM_SHOT=<png outside the repo> go test ./internal/surfaces/panelsim -run Shot`
renders the panel with sample values.
