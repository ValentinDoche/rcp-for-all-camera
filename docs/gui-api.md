# GUI API (Go ↔ frontend contract)

The GUI is a Wails v2 app. The Go type `app.Service` ([internal/app](../internal/app)) is
bound to the frontend; each exported method becomes a promise-returning JS function:

```js
window.go.app.Service.Rig().then(rig => …)
```

Events are pushed with the Wails runtime: `window.runtime.EventsOn(name, cb)`.

Go types and JSON field names are in [internal/app/api.go](../internal/app/api.go) and
[internal/core](../internal/core). This file is the reference; change both together.

## Methods

| Method | Returns | Notes |
|---|---|---|
| `PanelStatus()` | `PanelStatus` | Is the ATEM Micro Camera Panel plugged in |
| `Models()` | `ModelInfo[]` | Every camera model the app knows, in display order |
| `Rig()` | `Rig` | Always 8 slots, numbered 1–8 |
| `AssignSlot(slot, input)` | `Slot` | Create or replace the camera on slot 1–8 |
| `ClearSlot(slot)` | – | Remove the camera from a slot. Idempotent |
| `SelectSlot(slot)` | – | Make slot the active camera (what the panel drives). 0 = none |
| `Controls(slot)` | `ControlView[]` | Every control of the slot's camera, with last known value |
| `SetControl(slot, id, value)` | – | Write one control. Validated before any I/O |
| `ConnectSlot(slot)` | – | Start connecting the slot's camera |

## Events

| Name | Payload | When |
|---|---|---|
| `panel:status` | `PanelStatus` | Panel plugged / unplugged |
| `rig:changed` | `Rig` | Any slot assigned, cleared or selected, or a camera status change |
| `camera:event` | `{slot, event}` | A control value changed on a camera |

## Shapes

```ts
type PanelStatus = { connected: boolean; serial?: string; detail?: string }

type ModelInfo = {
  id: string            // "sony.fx6"
  brand: string         // "Sony"
  name: string          // "FX6"
  driver: string        // "sony"
  transports: ("ethernet" | "wifi" | "usbEthernet")[]
  defaultPort: number
  auth: "none" | "optional" | "required"   // show user/password fields unless "none"
  support: "planned" | "experimental" | "supported"
  notes?: string
}

type Rig  = { slots: Slot[]; active: number }        // active 0 = none
type Slot = { slot: number; camera: CameraView | null }

type CameraView = {
  model: string; brand: string; modelName: string; support: string
  name: string; host: string; port: number; transport: string
  username?: string; hasPassword: boolean          // password is never sent back
  status: { state: "disconnected" | "connecting" | "connected" | "error"; detail?: string }
}

type SlotInput = {
  model: string; name: string; host: string
  port: number                 // 0 = model default
  transport: string            // "" = model's first transport
  username: string
  password: string | null      // null = keep stored, "" = clear, else replace
}

type ControlSpec = {
  id: string; label: string
  group: "lens" | "exposure" | "white" | "color" | "image" | "ptz" | "transport" | "misc"
  kind: "range" | "enum" | "toggle" | "action"
  unit?: string; min?: number; max?: number; step?: number
  options?: { value: string; label: string }[]
  readOnly?: boolean
}

type Value = { kind: ControlSpec["kind"]; number?: number; choice?: string; on?: boolean }
type ControlView = { spec: ControlSpec; value: Value | null }   // null = unknown
```

Build values by kind: range `{kind:"range", number}`, enum `{kind:"enum", choice}`,
toggle `{kind:"toggle", on}`, action `{kind:"action"}`.

## Errors

A failed call rejects its promise with a string holding JSON:
`{"code": "INVALID_CONFIG", "message": "..."}`. Codes (stable, only added to):

`INVALID_SLOT`, `EMPTY_SLOT`, `INVALID_CONFIG`, `UNKNOWN_MODEL`, `UNKNOWN_CONTROL`,
`INVALID_VALUE`, `READ_ONLY`, `NOT_CONNECTED`, `NOT_IMPLEMENTED`, `INTERNAL`.

While camera protocols are not written, `ConnectSlot` and `SetControl` on a valid value
return `NOT_IMPLEMENTED`; the GUI shows that state instead of failing silently.
