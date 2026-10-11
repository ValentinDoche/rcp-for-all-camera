# App API (`app.Service`)

`app.Service` ([internal/app](../internal/app)) is the only entry point the GUI uses. The
GUI is native Go (Fyne, [internal/gui](../internal/gui)) and calls these methods directly.
The same API can later back a headless runner or a remote control surface.

`app.Host` owns the lifecycle (event emitter, panel polling, shutdown). It is separate so
that the API surface stays limited to the methods below.

Types are in [internal/app/api.go](../internal/app/api.go) and [internal/core](../internal/core).
JSON tags are kept on every type so the API can be exported or logged in a stable form.

## Methods

| Method | Returns | Notes |
|---|---|---|
| `PanelStatus()` | `PanelStatus` | Panel state: `connected` (plugged in), `active` (driven by the app), `serial`, `detail` (see below) |
| `Models()` | `[]core.ModelInfo` | Every known camera model, in display order |
| `Rig()` | `Rig` | Always 8 slots, numbered 1 to 8, and the active slot (0 = none) |
| `AssignSlot(slot, SlotInput)` | `Slot` | Create or replace the camera on a slot |
| `ClearSlot(slot)` | | Remove the camera from a slot. Idempotent |
| `SelectSlot(slot)` | | Make a slot active (what the panel drives). 0 = none. An empty slot cannot be selected (`EMPTY_SLOT`) |
| `Controls(slot)` | `[]ControlView` | Every control of the slot's camera with its last known value (nil = unknown) |
| `SetControl(slot, id, core.Value)` | | Write one control. Validated before any I/O |
| `ConnectSlot(slot)` | | Start connecting the slot's camera |

`SlotInput.Password`: nil keeps the stored password, `""` clears it, anything else replaces
it. Passwords are never returned; `CameraView.HasPassword` says whether one is stored.

## Events (`app.Emitter`)

| Name | Payload | When |
|---|---|---|
| `panel:status` | `PanelStatus` | Panel plugged, unplugged, driven or released (sent on change only) |
| `rig:changed` | `Rig` | A slot was assigned, cleared or selected |
| `camera:event` | `CameraEvent{Slot, Event}` | A camera changed state (`Event.Status`) or a control value (`Event.Control`, `Event.Value`) |

Emit is called from service goroutines; the GUI must hop to its UI thread.

## Values

`core.Value` carries one field per control kind: `Number` (range), `Choice` (enum), `On`
(toggle), nothing (action). Build them with `core.Number`, `core.Choice`, `core.Bool`,
`core.Trigger`. In JSON, zero values are omitted (`omitempty`).

## Errors

Every method returns `*app.Error` on failure (`errors.As`). `Code` is stable and only ever
added to; `Message` is for logs and may change. Unknown internal errors map to `INTERNAL`
with a generic message, so paths and internals never reach the GUI.

Input bounds (see [security.md](security.md)): control ids are 1 to 64 characters of
`[A-Za-z0-9._-]`, enum choices are at most 64 printable characters, and model ids are at
most 64 bytes. Names and credentials must be printable UTF-8 with no control, bidi or
zero-width characters.

`INVALID_SLOT`, `EMPTY_SLOT`, `INVALID_CONFIG`, `UNKNOWN_MODEL`, `UNKNOWN_CONTROL`,
`INVALID_VALUE`, `READ_ONLY`, `NOT_CONNECTED`, `NOT_IMPLEMENTED`, `INTERNAL`.

While camera protocols are not written, `ConnectSlot` and `SetControl` with a valid value
return `NOT_IMPLEMENTED`; the GUI shows this as "driver in development", not as a failure.

## Panel status

| `connected` | `active` | `detail` | Meaning |
|---|---|---|---|
| false | false | "" | No panel plugged in |
| false | false | "détection USB indisponible" | USB enumeration failed |
| true | false | "" | Plugged in, being opened (or between retries) |
| true | false | "utilisé par un autre logiciel (fermez ATEM Software Control)" | ATEM Software Control is running; the app leaves the panel alone and retries |
| true | false | "panneau détecté mais inaccessible, nouvel essai…" | Open or authentication failed; retried with backoff |
| true | true | "" | Driven by the app: displays show the active camera, controls work |

`active` was added on 2026-10-11 (additive). The panel mapping is in
[panel-mapping.md](panel-mapping.md).

## Control surfaces (Host)

`Host.SetSurface(app.Surface)` (before `Start`) registers a control surface. The Host runs
`Surface.Run(ctx, SurfaceAPI, report)` and forwards every Service event to the surface
through `Emit`, next to the GUI emitter. `Host.Subscribe(Emitter)` adds any other listener.
`SurfaceAPI` is the subset of Service a surface uses: `Rig`, `Controls`, `SetControl`,
`SelectSlot`. Surfaces never call `ConfirmSetControl`.
