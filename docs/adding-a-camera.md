# Adding a camera

Every camera model is a **module**: a small Go program under `modules/<id>/`, compiled to
WebAssembly (`<id>.wasm`) and loaded from the `modules` folder next to `rcp.exe`. Modules are
independent: each one imports only the SDK, `sdk/rcpmod`, and never another module.

The full guide, written for someone who has never seen the project, is
**[Camera modules](modules-camera.html)**: concepts, a quick start from the demo module, the
module contract, every control, the UI file (`ui.go`), the block and widget galleries, testing,
the sandbox and the ABI. This page is the short version for contributors.

## The files of a module

| File | Holds |
|---|---|
| `main.go` | `rcpmod.Register(module)` in `init`, the model description (`rcpmod.Info`) and the controls (`[]rcpmod.ControlSpec`) |
| `ui.go` | The camera page: `rcpmod.StandardPage()`, or a custom page with extra rows (`AddRow`, `Controls`, `Grid`, `Card`, `Wheel`, `Remaining`) |
| `*_test.go` | The contract (`rcpmodtest.Module`, and `rcpmodtest.Online` against a fake camera) plus what is specific to the model |
| other files | The protocol, named after what they hold (`studio_camera.go`, `rest_properties.go`) |

## Add a model whose protocol is not written yet

1. Copy a planned module (for example `modules/sony-fx6`) to `modules/<id>/`. The id is
   `<brand>-<model>` in lowercase, and the folder has the same name.
2. Fill `rcpmod.Info` (`Support: rcpmod.SupportPlanned`) and the controls, with the standard ids
   (`rcpmod.Iris`, `rcpmod.Gain`...). Use a brand-prefixed id (`sony.baseISO`) only when no
   standard id fits. Mark every range you did not verify with `// TODO(verify): ...`.
3. Keep `rcpmod.Planned{Model: info, Specs: specs, Page: page}`: the page shows every control,
   and connecting or writing answers "not implemented".
4. `go run ./cmd/buildmodules <id>`, then `go test ./modules/<id>/`.

## Write the protocol

1. Replace `rcpmod.Planned` with your own `rcpmod.Module` and `rcpmod.Camera` (see
   `modules/bm-studio-6k-pro` for a real one, `modules/demo` for a simulated one). Keep the id
   and the controls.
2. Reach the camera only through the `rcpmod.Host` services (HTTP, WebSocket, timers, log): the
   sandbox allows nothing else, and only towards the camera's configured address.
3. Test against a fake camera server with `rcpmodtest.Online`, then on the real camera with an
   opt-in live test (`BMD_CAMERA=<ip>`-style environment variable).
4. Raise `Support` to `experimental`, then `supported` once verified on hardware.

## Rules

- `New` does no I/O. Connecting is `Connect`'s job.
- `Set` validates against the declared control before any I/O; the host checks it again before
  calling the module.
- Never silently drop a command: a failure on the wire returns an error.
- A camera that cannot be reached is `disconnected` with a reason for the user, and stays so
  while the module retries. `error` is for a camera that answers but refuses; the panel's ALARM
  blinks on it.
- Credentials are never logged, never put in an error or a status detail.
- Protocol details (URLs, JSON, byte layouts) stay inside the module. Everything outside speaks
  controls and values.
- Log the commands sent to the camera at debug level.
- Ask before sending anything that could change recording or output on live gear.
- A model that takes any lens sets `Info.InterchangeableLens`, and declares `rcpmod.Zoom` only
  when `Config.PowerZoom` is set. A built-in zoom (PTZ) leaves both alone.
- A simulated camera with nothing to reach sets `Info.NoAddress`: the form hides host and port.
- An iris in f-stops uses unit `"f"`. If the camera knows its lens, report it with
  `rcpmod.LensEvent`: the GUI then shows the focal length and the focus range, and keeps the iris
  inside the lens' limits.

## Checklist

- [ ] The folder name and `Info.ID` match (`<brand>-<model>`), and are unique
- [ ] Transports, default port and auth are right; `Notes` is one English sentence
- [ ] Controls cover what the camera's API exposes, with the standard ids
- [ ] Unverified ranges carry `TODO(verify)`
- [ ] `ui.go` describes the page; `Page.Check` passes (the contract runs it)
- [ ] `rcpmodtest.Module` passes, plus model-specific tests
- [ ] `gofmt`, `go vet ./...`, `go test ./...` pass
