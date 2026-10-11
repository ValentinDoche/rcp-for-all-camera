# ATEM Micro Camera Panel — USB HID protocol

Reference for the `surface-micro-panel` module ([internal/surfaces/micropanel](../internal/surfaces/micropanel)).
Recovered on 2026-10-11 from live USB captures of the owner's panel and static
analysis of `ATEM Software Control.exe` (ATEM Switchers 10.x install, Windows).
How it was found and what is still open: [reports/2026-10-11-micro-camera-panel-usb-spike.md](reports/2026-10-11-micro-camera-panel-usb-spike.md).

Status legend: **✅ confirmed on hardware** · **🔎 from static analysis, not yet tested** · **❓ unknown**

## 1. USB identity

| Item | Value |
|---|---|
| Vendor / product | `0x1EDB` (Blackmagic Design) / `0xBF01` |
| Interface MI_00 | class *MEDIA*, bound to Blackmagic's ATEM driver (`Switchers.inf`). Not used for panel I/O. |
| Interface MI_02 | **HID**, usage page `0xFF00` (vendor). All panel I/O goes here. Standard Windows HID driver, no Blackmagic driver needed. |
| Serial | USB serial string (e.g. `51FAF46D3FCF41928805487C9118E9FF`) |
| Max report sizes (HidP_GetCaps) | input 19 B, output 33 B, feature 64 B (all including the report ID byte) |

The panel works with **no ATEM switcher** present: it only needs the host to authenticate. ✅

> **Feature reports must be sent/read with their exact size.** Padding a
> `HidD_GetFeature`/`HidD_SetFeature` buffer to the 64-byte maximum makes the
> panel stall the request (`ERROR_GEN_FAILURE`, "A device attached to the system
> is not functioning"). Output reports via `WriteFile` are padded to 33 bytes
> without problem. ✅

## 2. Report map

| ID | Dir | Size (with ID) | Meaning | Status |
|---|---|---|---|---|
| `0x02` | out | 32 | Button / indicator LEDs (bitmap + layer) | ✅ |
| `0x03` | in | 13 | Held keys: up to 6 × u16 key codes | ✅ |
| `0x04` | feature | 8 | Status/version? Bytes change between reads (`04 20 15 41…`, `04 60 1f 41…`) | ❓ |
| `0x05` | feature | 10 | Authentication (challenge/response) | ✅ |
| `0x06` | in + feature | 3 | Battery: byte 1 bit 0 = charging, byte 2 = percent (reads `00 64`) | 🔎 |
| `0x09` → `0x0A` | feature | 4 / 28 | Set `09 00 00 00`, then read `0A`: `[a, b, c, u32, 10 × u16]`, the u16s are the switcher input IDs assigned to the camera buttons (`1..8, 0, 0`) | 🔎 |
| `0x0B` | feature | 14 | ❓ | ❓ |
| `0x0D` | in | 5 | `settingsChanged` flags (u32); host then reads feature `0x14` | 🔎 |
| `0x0E` | in + feature | 19 | 9 × u16 absolute knob positions 0..1024 | ✅ |
| `0x0F` | in + feature | 13 | 3 × int32: `[1, iris lever, master-black ring]`, ±4096 | ✅ |
| `0x10` | out | 15 | 14 seven-segment digits, 1 raw byte each | ✅ |
| `0x11` | out | 33 | Camera-number dot matrix, 2 bits/pixel | ✅ |
| `0x12` | feature | 4 | Scene-file directory: set `[1, cam, 0]`, read `[x, cam, bits]`, bits 0..4 = slots stored | 🔎 |
| `0x13` | feature | 64 | Scene file: `[slot 1..5, camera 1..80, 61-byte blob]`; stored **in the panel** | 🔎 |
| `0x14` | feature | 2 | 1 bit setting (read after `0x0D`) | ❓ |
| `0x15` | feature | 33 | 32-byte ASCII, probably device name | 🔎 |

## 3. Authentication (feature report 0x05) ✅

The panel sends **no input reports** until the host authenticates. It uses the same
challenge/response as Blackmagic's DaVinci Speed Editor (algorithm published by the
community, e.g. `smunaut/blackmagic-misc`), on report ID `0x05` instead of `0x06`.
Payload: `[05, cmd, u64 little-endian]`.

```
SET 05 00 00×8           → GET 05: 05 00 <device challenge, u64 LE>
SET 05 01 00×8           → GET 05: 05 02 …          (host challenge; reply not checked by us)
SET 05 03 <F(chal) LE>   → GET 05: 05 04 <u16 LE timeout s> …  (04 = OK, panel answers 600 s)
```

`F` is implemented in [auth.go](../internal/surfaces/micropanel/auth.go) and
tested against three challenges the real panel accepted ([auth_test.go](../internal/surfaces/micropanel/auth_test.go)).
Re-authenticate before the timeout runs out (ATEM Software Control uses min(timeout/2, 900 s); we use 400 s).

ATEM Software Control also verifies the panel's answer to the host challenge
(mutual auth, a second function F2). A host that only wants to drive the panel can
skip that check.

## 4. Inputs

### 4.1 Buttons — report 0x03 ✅

`03 k0 k0 k1 k1 … k5 k5`: up to six u16 LE codes of the keys **currently held**
(0 = empty slot). Press and release are found by diffing two reports. All keys
released → all zero.

| Code | Control | | Code | Control |
|---|---|---|---|---|
| `0x40`–`0x47` | Camera 1–8 | | `0x52` / `0x53` | MASTER GAIN ▲ / ▼ |
| `0x48`–`0x4C` | SCENE FILE 1–5 | | `0x54` / `0x55` | SHUTTER ▲ / ▼ |
| `0x4D` | STORE | | `0x56` | ON |
| `0x4E` / `0x4F` | ND ▲ / ▼ | | `0x57` | W/B |
| `0x50` / `0x51` | CC ▲ / ▼ | | `0x58` | BARS |

Bottom-area buttons. The codes are certain, but **the names are inferred from the order they were pressed in and are not confirmed yet**:

| Code | Probable control |
|---|---|
| `0x59` | Black/Flare square button (not seen in the capture) |
| `0x5A` | IRIS/MB ACTIVE |
| `0x5B` | AUTO IRIS |
| `0x5C` | CALL |
| `0x5D` | PANEL ACTIVE |
| `0x5E` | PREVIEW |

### 4.2 Knobs — report 0x0E ✅

`0E v0 … v8`: 9 × u16 LE, **absolute** position 0..1024. The panel integrates the
encoder itself and clamps at 0 and 1024. Because the report is also a *feature*,
the host can read the current positions at any time, and very likely write them
to sync the knobs with a camera's real value (writing is not tested yet).

| Index | Control |
|---|---|
| 0 | WHITE R |
| 1 | BLACK R |
| 2 | WHITE G |
| 3 | BLACK G |
| 4 | WHITE B |
| 5 | BLACK B |
| 6 | Top-right knob (no label) |
| 7 | SENS |
| 8 | COARSE |

### 4.3 Iris lever and master-black ring — report 0x0F ✅

`0F m m m m L L L L R R R R`: three int32 LE.
- `m`: always 1 in captures (a mode?)
- `L`: iris lever position, −4096 (closed / pulled back) to +4096
- `R`: master black ring, roughly −4096 to +4096

ATEM Software Control divides these values by 4096.

## 5. Outputs

### 5.1 Seven-segment digits — report 0x10 ✅

`10 d0 … d13`: one byte per digit, raw segments:

```
bit7 bit6 bit5 bit4 bit3 bit2 bit1 bit0
 a    b    c    d    e    f    g    DP
```

| Digits | Display |
|---|---|
| 0 | ND |
| 1 | CC (ATEM Software Control never writes it) |
| 2–3 | MASTER GAIN |
| 4–7 | SHUTTER |
| 8–10 | IRIS (e.g. `F` `2.` `8`) |
| 11–13 | MASTER BLACK |

Blackmagic's glyphs: `0`=FC `1`=60 `2`=DA `3`=F2 `4`=66 `5`=B6 `6`=BE `7`=E0 `8`=FE `9`=F6,
`F`=8E `C`=9C `L`=1C `S`=B6 `A`=EE `u`=38 `t`=1E `o`=3A, `-`=02, `.`=01.
`EE 38 1E 3A` on digits 4–7 spells **Auto**. Encoder: [display.go](../internal/surfaces/micropanel/display.go).

### 5.2 Camera dot matrix — report 0x11 ✅

`11 b0 … b31`: **7 rows × 10 columns**, 2 bits per pixel. Pixel `p = row*10 + col`
is in byte `p >> 2`, bits `2*(p & 3)` (LSB first). Only 70 pixels are used, and bytes 18–31 stay 0.

| Value | Colour |
|---|---|
| 0 | off |
| 1 | red ✅ |
| 2 | green 🔎 |
| 3 | amber 🔎 |

Numbers 1–9 use a 5-pixel-wide centred font. Numbers 10–99 use two 4-pixel-wide
digits, the second shifted 5 columns. Fonts: [matrix.go](../internal/surfaces/micropanel/matrix.go).

### 5.3 LEDs — report 0x02 ✅

`02 <30 bytes bitmap> <layer 0..5>`: LED *n* = byte `n >> 3`, bit `n & 7`.
All panel LEDs are on **layer 0** and indices **0–74**. The meaning of layers
1–5 is unknown (blink or brightness?). Mapped on hardware with seven binary-coded
photos, then checked with a confirmation pattern. Constants are in [leds.go](../internal/surfaces/micropanel/leds.go).

| LED | Function |
|---|---|
| 0–15 | Iris scale bar graph, 0 = top (above 2.0) … 15 = CLS. Labels 2.0, 2.8, 4.0, 5.6, 8.0, 11, 16 and CLS sit on segments 1, 3, 5 … 15 (two segments per stop) |
| 16+3c, 17+3c, 18+3c | Camera button c+1 (c = 0..7): **R, G, B** channels (amber = R+G, cyan = G+B, white = all three) |
| 40 | SCENE FILE indicator |
| 41–45 | SCENE 1–5 backlight |
| 46 / 47 | STORE white / red |
| 48 / 49 | ND ▲ / ▼ |
| 50 / 51 | CC ▲ / ▼ |
| 52 / 53 | MASTER GAIN ▲ / ▼ |
| 54 / 55 | SHUTTER ▲ / ▼ |
| 56 / 58 / 60 | Indicators REL / ABS / BARS |
| 57 / 59 / 61 | ON / W/B / BARS button backlights |
| 62 / 63 | D EXT / EXT indicators |
| 64 / 65 | IRIS/MB ACTIVE white / red |
| 66 | AUTO IRIS |
| 67 / 68 | CALL white / red |
| 69 / 70 / 71 | NETWORK / ALARM / CABLE |
| 72 / 73 | PANEL ACTIVE white / red |
| 74 | PREVIEW |

The square BLACK/FLARE button has no LED.

**Layers persist in the panel.** Each layer keeps its bitmap until it is
rewritten, and survives a host restart. A bit set on any layer lights the LED, so
treat the result as an OR of the layers. A host that only drives layer 0 must
clear layers 1–5 at start-up, otherwise a stale bit stays lit. This happened in
testing: iris segment 11 stayed lit after the LED explorer had used another layer.
`micropanel.Open` clears all six layers.

Setting bits **above 74** made the camera matrix draw unexpected shapes during
the mapping tests. Keep bits 75–239 at 0.

How ATEM Software Control uses them: program = red, preview = green, selected = cyan, selected and on preview = amber.
It lights the ▲/▼ arrow pair according to whether the value can still move (48/49, 54/55).

## 6. Tools

- [`cmd/panelprobe`](../cmd/panelprobe/main.go): authenticates, dumps raw input reports, writes raw output/feature reports. Example: `go run ./cmd/panelprobe -out 10:fc60dafe… -t 0`
- [`cmd/paneltest`](../cmd/paneltest): interactive test bench, see [paneltest.md](paneltest.md).
