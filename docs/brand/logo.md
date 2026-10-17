# RCP For All Camera logo

![Logo](../../internal/gui/brand/logo.svg)

Files:

| File | Use |
|---|---|
| [internal/gui/brand/logo.svg](../../internal/gui/brand/logo.svg) | App icon: the mark on its graphite tile (window, title bar, taskbar) |
| [internal/gui/brand/mark-dark.svg](../../internal/gui/brand/mark-dark.svg) | Mark alone, on a dark background |
| [internal/gui/brand/mark-light.svg](../../internal/gui/brand/mark-light.svg) | Mark alone, on a light background (graphite blades) |
| [logo-construction.svg](logo-construction.svg) | Construction drawing (grid, circles, guide lines) |
| [gen_logo.py](gen_logo.py) | The generator: every dimension is in it. Change the script, never the SVGs by hand |

```bash
python docs/brand/gen_logo.py   # from the repository root
```

## The idea

The logo is a **camera iris with 8 blades**, **only one of them orange**, inside a **ring**.

- **The iris** is what an RCP controls. Every brand (Blackmagic, Sony, Panasonic) has an
  aperture: it is the setting all cameras share, the "For All". It is also the operator's main
  gesture on the Micro Camera Panel: the iris lever.
- **8 blades = the panel's 8 camera buttons** (Cam1 to Cam8, the app's 8 slots).
- **One orange blade = the selected camera.** It is the app's selection orange (`style.Accent`,
  `#F47B20`) and the colour of the active camera button on the panel. The other 7 blades wait:
  one camera is controlled at a time, among all of them.
- **The ring** around the blades recalls the master black ring around the panel's iris lever,
  and a lens ring. It holds everything together: one desk around all the cameras.
- **The octagonal opening** in the centre is the light passing through: the camera "open" to
  the operator.

The colours are those of the interface: graphite (`Surface`), light grey (`Text`), ring grey
(`TextFaint`) and the selection orange. The logo says nothing about any camera brand and copies
no existing logo.

## Construction

![Construction](logo-construction.svg)

Everything is drawn on a **64 × 64** unit grid, centre `(32, 32)`.

1. **Tile**: a 64 × 64 square, corner radius **14**, colour `#212225`. It is drawn as a path
   (arcs) rather than `<rect rx>`, which Fyne's SVG renderer ignores.
2. **Ring**: a circle of radius **26**, stroke **2**, colour `#6B6F78`, no fill. A 5-unit margin
   between the ring and the edge of the tile.
3. **Opening**: a regular octagon inscribed in a circle of radius **r = 8**. Its first vertex is
   at −112.5° (−90° − 22.5°), which puts a full blade at 12 o'clock.
4. **Blades**: each side of the octagon is extended (red guide lines) to the circle of radius
   **R = 21**. Two consecutive sides and the arc between them outline a blade: each blade is a
   triangle whose tip is a vertex of the octagon and whose long side is an arc. 8 blades, one
   every **45°**.
5. **Gaps between blades**: each side is offset **0.8** into its blade (a total gap of **1.6**).
   The blade tip becomes the intersection of the two offset sides, which very slightly enlarges
   the opening. The gaps are real holes (not a stroke in the background colour): the mark works
   on any background.
6. **Colours**: 7 blades `#E8E9EC`, the 12 o'clock blade `#F47B20`. Light version: blades
   `#2B2D31`, ring `#7A7F89`, orange blade unchanged.

Ratios to keep when redrawing: R/r ≈ 2.6 (below, the iris looks closed and fills in at small
sizes; above, the blades turn into threads), ring = R + 5, ring stroke ≈ gap between blades.

## Usage rules

- **Minimum size**: 16 px (window icon). Below 24 px, prefer the tile to the mark alone: the
  graphite background keeps the blades' contrast.
- **Clear space**: 1/8 of the width around the tile (8 units out of 64).
- **Only one orange blade**, always at 12 o'clock. Do not animate the colour onto another blade
  and do not colour several blades: orange means "the selected camera".
- Do not distort it, do not rotate it (the orange blade would leave 12 o'clock), add no shadow
  or gradient, and do not replace the orange with the tally red (`#CC2621`): red means "on air".
- The name is written **RCP For All Camera**, to the right of the icon, in the interface font,
  in bold.
