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
| `Rig()` | `Rig` | Always 8 slots, numbered 1 to 8, and the active slot (0 = none). Each `Slot` has `presets`: the stored preset numbers, sorted (`[]` when none) |
| `AssignSlot(slot, SlotInput)` | `Slot` | Create or replace the camera on a slot |
| `ClearSlot(slot)` | | Remove the camera from a slot. Idempotent |
| `SelectSlot(slot)` | | Make a slot active (what the panel drives). 0 = none. An empty slot cannot be selected (`EMPTY_SLOT`) |
| `Controls(slot)` | `[]ControlView` | Every control of the slot's camera with its last known value (nil = unknown) |
| `Lens(slot)` | `core.Lens` | What the camera reports about its lens: f-stop limits, focal length and range (mm), focus distance and range (m). Zero fields are unknown; zero for cameras that are not a `core.LensReporter` or are offline |
| `SetControl(slot, id, core.Value)` | | Write one control. Validated before any I/O |
| `ConnectSlot(slot)` | | Start connecting the slot's camera |
| `StorePreset(slot, n)` | | Snapshot the camera's storable controls as preset `n` (1 to `PresetCount` = 5), replacing any preset there |
| `RecallPreset(slot, n)` | | Write preset `n` back to the camera |
| `DeletePreset(slot, n)` | | Remove preset `n`. Idempotent |
| `SetStoreArmed(on)` | | Arm or disarm "Store" (global, shared by the panel's STORE key and the GUI's Store button). Emits `preset:storeArmed` only on change |
| `StoreArmed()` | `bool` | Whether Store is armed |

`SlotInput.Password`: nil keeps the stored password, `""` clears it, anything else replaces
it. Passwords are never returned; `CameraView.HasPassword` says whether one is stored.

`SlotInput.PowerZoom`: the mounted lens has a motorised zoom. It only counts for models
with `ModelInfo.InterchangeableLens` (it is stored as false for the others). Without it,
the camera declares no `zoom` control. `CameraView` returns `powerZoom` and
`interchangeableLens` so the form can show the setting.

### Presets

A preset is a snapshot of a camera's control values, stored by the app in `rig.json`
(`presets`) and recalled by writing the values back with `Camera.Set`. Presets are
**never stored on the camera** (the Blackmagic camera's own presets drop color
correction), so they work the same with every driver.

- Storable controls: not read-only, not an action, not guarded, not in the `ptz` group
  (pan/tilt are speeds and `presetRecall` is a PTZ position: recalling them would move the
  camera). Guarded controls are never written, so the operator lock does not apply.
- `StorePreset` needs a connected camera (`NOT_CONNECTED`). It reads every storable
  control under one 3 s deadline; a control that cannot be read is left out, and if none
  can be read the call fails with `NOT_CONNECTED`.
- `RecallPreset` writes in the camera's `Controls()` order, only the controls that still
  exist and are still storable. A control that fails is skipped and the others are still
  written; the first error is returned after all were tried. `EMPTY_PRESET` when nothing
  is stored under `n`, `INVALID_PRESET` when the preset was taken on another model.
- A preset belongs to the slot and to the camera model. Assigning a different model to the
  slot, clearing the slot, or importing a configuration that changes the slot's model drops
  its presets; editing the name, address or credentials of the same model keeps them.
- Store arming is one global flag. While armed, the next preset number pressed stores
  instead of recalling. `StorePreset` always disarms afterwards, success or failure (the
  gesture is over), so a failed store never leaves a surprise store armed.
- Store and delete emit `rig:changed`, so the GUI gets the new `presets` list.
- At startup, saved presets are dropped with a warning when their slot or number is out of
  range, the slot is empty, or the model differs; for duplicates the last one wins, and
  values with an invalid control id or value are dropped.

## Events (`app.Emitter`)

| Name | Payload | When |
|---|---|---|
| `panel:status` | `PanelStatus` | Panel plugged, unplugged, driven or released (sent on change only) |
| `rig:changed` | `Rig` | A slot was assigned, cleared or selected, or a preset was stored or deleted |
| `preset:storeArmed` | `StoreArmedState{Armed}` | The shared Store arming changed (only on a real change) |
| `camera:event` | `CameraEvent{Slot, Event}` | A camera changed state (`Event.Status`), a control value (`Event.Control`, `Event.Value`) or its lens information (`Event.Lens`) |
| `camera:commandFailed` | `CommandFailed{Slot, Control, Code}` | A control write failed after reaching the slot's camera (from any surface, or a preset recall). The panel flashes ALARM |

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
`INVALID_VALUE`, `READ_ONLY`, `NOT_CONNECTED`, `NOT_IMPLEMENTED`, `INTERNAL`,
`CONFIRM_REQUIRED`, `EMPTY_PRESET` (no preset stored under that number), `INVALID_PRESET`
(number outside 1 to 5, or preset taken on another camera model).

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
