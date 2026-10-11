# Adding a camera

How camera support is organised, and how to add a brand or a model.

## How it works

- `core.Driver` (one per protocol family: `blackmagic`, `sony`, `panasonic`) declares
  `Models()` and builds cameras with `New(model, cfg)`.
- `core.ModelInfo` describes one model: id (`<driver>.<model>`), brand, name,
  transports (`ethernet`, `wifi`, `usbEthernet`), default port, auth mode
  (`none`/`optional`/`required`), support level (`planned`/`experimental`/`supported`)
  and a short note (French, shown in the GUI).
- `core.ControlSpec` describes one parameter: id, label, group, kind (`range`, `enum`,
  `toggle`, `action`), unit, min/max/step, options, read-only. A control that is not
  declared is not supported by that model.
- `core.Std(id)` returns a spec with the standard label and group for the ids in
  `internal/core/catalog.go`. Set `Kind`, range and options on top of it. The helpers in
  `internal/cameras/speckit` (`Range`, `Enum`, `Toggle`, `Action`, `ReadOnly`, `Opts`,
  `Labeled`, `Numbered`) do this in one line. Use a driver-prefixed id
  (`sony.baseISO`) only when no standard id fits.
- `camerakit.Placeholder` is a `core.Camera` that declares its controls but has no
  protocol: `Set` validates then returns `ErrNotImplemented`. Models start with it, so the
  GUI can show and validate every parameter before any protocol code exists.
- `cameratest.Driver(t, Driver{})` is the contract suite every driver must pass.
- `cameras.Register` (`internal/cameras/all.go`) registers all drivers explicitly.

## Add a model to an existing driver

1. Add a `ModelInfo` in the driver's `Models()` (`Support: core.SupportPlanned`).
2. Add a case in `specsFor` that calls the family spec builder with the right options
   (`hasND`, `ptz`, ...). If the model differs from its family, add an option to the
   builder rather than copying it.
3. Mark every range you did not verify with `// TODO(verify): ...`.
4. Add a test for what is specific to the model (has ND, has pan/tilt, auth mode).
5. `gofmt -w`, `go vet ./...`, `go test ./...`.

## Add a new brand

1. Create `internal/cameras/<brand>/` with `type Driver struct{}`; `Name()` is the id
   prefix of all its models. Write the package doc comment: protocol, port, auth.
2. Write tests first: `func TestContract(t *testing.T) { cameratest.Driver(t, Driver{}) }`
   plus facts about the models. See them fail.
3. Implement `Models()`, `New()` (returns `camerakit.NewPlaceholder(specs)`) and the
   spec builders.
4. Register the driver in `cameras.Register` and update `all_test.go` (model count).
5. Later, replace the placeholder with a real `core.Camera` implementing the protocol.
   Keep the same `Controls()`; raise `Support` to `experimental`, then `supported` once
   verified on hardware.

## Rules

- `New` does no I/O. Connecting is `Camera.Connect`'s job.
- `Set` validates against the spec (`ControlSpec.Validate`) before any I/O and returns
  `ErrUnknownControl`, `ErrInvalidValue` or `ErrReadOnly` without touching the network.
- Never silently drop a command: a failure on the wire returns an error.
- Credentials are never logged, never serialized, never put in error messages or status
  details.
- Protocol details (URLs, JSON, byte layouts) stay inside the driver package. Everything
  outside speaks `ControlID` and `Value`.
- Log the commands sent to the device at debug level.
- Ask before sending anything that could change recording or output on live gear.
- No new runtime dependency without asking.
- A model that takes any lens sets `ModelInfo.InterchangeableLens`, and `New` declares
  `core.Zoom` only when `CameraConfig.PowerZoom` is set. A built-in zoom (PTZ) leaves
  both alone.
- An iris in f-stops uses unit `"f"`. If the camera knows its lens, implement
  `core.LensReporter` and push `core.Event{Lens: ...}` on change: the GUI then shows the
  focal length, the focus range and keeps the iris inside the lens' limits.

## Checklist

- [ ] Model ids are `<driver>.<model>` and unique
- [ ] Transports, default port and auth are right; note is one French sentence
- [ ] Controls cover everything the camera's API exposes, with the standard ids
- [ ] Unverified ranges carry `TODO(verify)`
- [ ] Contract suite passes, plus model-specific tests
- [ ] Driver registered in `cameras.Register`
- [ ] `gofmt`, `go vet ./...`, `go test ./...` pass
