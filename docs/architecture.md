# Architecture

```
cmd/rcp ─────────────► internal/gui (Fyne, native)
   │                        │ calls
   │                        ▼
   ├──────────────► internal/app  Service (API) + Host (lifecycle)
   │                   │        │          │
   │                   ▼        ▼          ▼
   │            internal/config  internal/core  internal/surfaces/micropanel
   │            (rig.json,       (contracts,    (USB HID driver,
   │             secrets)         registry)      presence probe)
   │                                 ▲
   └──────────────► internal/cameras ┘  Register(): blackmagic, sony, panasonic
                       camerakit (Placeholder, FindSpec)
                       speckit   (spec builders)
                       cameratest (contract suite)
```

Rules:

- `core` depends on nothing. Every other module depends on `core`, never on each other,
  except `app`, which wires them, and `gui`, which only talks to `app`.
- Camera modules are registered explicitly in `cameras.Register`. Adding a camera means a
  new package plus one line there; see [adding-a-camera.md](adding-a-camera.md).
- A camera declares all its controls as `core.ControlSpec`. The GUI renders any spec it
  receives, so a new parameter needs no GUI change.
- The panel driver keeps protocol details inside `internal/surfaces/micropanel`.
- Passwords live only in the secrets store, never in `rig.json`, logs or the GUI.

## Status (2026-10-11)

| Module | State |
|---|---|
| `core`, `config`, `app` | Done, unit tested |
| `cameras/*` | Models and control specs declared; no protocol (Placeholder) |
| `surfaces/micropanel` | Driver done and hardware verified; presence detection used by the GUI; not yet wired to cameras |
| `gui` | Fyne native GUI |
| routers (Videohub, ATEM, vMix) | Not started |

See [app-api.md](app-api.md) for the API the GUI uses and [security.md](security.md) for the
security model.
