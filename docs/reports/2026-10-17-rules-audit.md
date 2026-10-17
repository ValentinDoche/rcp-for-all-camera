# Code audit against the Go engineering rules (2026-10-17)

Reference: [docs/go-engineering-rules.md](../go-engineering-rules.md). For every gap found: the rule,
where it was, and what was decided for 0.3.0-ALPHA. **Fixed** means done and covered by tests.
**Kept** means the gap is accepted on purpose, with its reason (rule zero: a rule can be broken
knowingly).

## Contradictions with the original request

| Request | Rule | Outcome |
|---|---|---|
| Modules of one brand share a base (`bm`) and "inherit" from it, with hooks for camera-specific behaviour. | C11 (composition over inheritance), M5 and X3 (no dependency between modules). | Go has no inheritance; a shared base would also tie modules together. The owner then decided that **every module is independent**: no shared base, no hooks. Each module is a separate WASM program that depends only on the SDK. |
| Camera modules loaded from a `modules` folder next to the executable. | S2 (trust no data), S5 (no I/O in the core), S6 (few dependencies). | Modules are WebAssembly plugins run by wazero (approved as a new dependency) in a sandbox: no file system, no environment, network only to the configured camera, memory, time and message limits. A module that crashes stops only its own camera. |

## Gaps fixed

| # | Rule | Gap found | Fix |
|---|---|---|---|
| 1 | M5, X3 | The `demo` camera imported the Blackmagic driver to reuse its controls. | Every module is its own program under `modules/<id>/` and imports only `sdk/rcpmod`. The architecture test fails if a module imports anything else from the repository. |
| 2 | A2, C4 | One driver served several models, choosing with a `switch` on the model id. | One module per model, behind the `core.Module` port. |
| 3 | A1, A2 | `app` (application layer) imported `config` (infrastructure). | The `Store` and `Secrets` ports and the saved model (`app.SavedRig`) are declared in `app` (`persistence_ports.go`); `config` implements them. |
| 4 | H4, I7, D19 | Domain types carried `json` tags and `rig.json` was written straight from them. | No `json` tag left in `core`. `config` has its own persistence DTOs and mappers (`rig_file_store.go`); the plugin host maps the wire format to `core` types and validates it (`wire_mappers.go`), an anti-corruption layer. |
| 5 | C6, D30 | Global logger (`slog.Default()`) in a driver. | The logger is injected; modules log through the host (`rcp.log`). |
| 6 | C6 | Mutable package variables (`app.commandTimeout`, `gui.Version`, `gui.MinWindow`, `widgets.LeverEvery`). | Options or constants. Package-level variables left are read-only tables, embedded files, and the build version set by the linker. |
| 7 | U4 | `Service.AssignSlot`, a command, returned data. | It returns only an error; the caller reads `Rig()` again. |
| 8 | A2 | The GUI (inbound adapter) called `sysinfo` (outbound adapter). | The GUI receives a `StatsSampler` port; `cmd/rcp` adapts `sysinfo` to it. |
| 9 | X1, X2 | No automatic check of the dependency direction. | `internal/archtest/dependency_rules_test.go`: `core` imports no other internal package, nor `net/http`, nor Fyne; `app` imports no adapter; a module imports only the SDK. |
| 10 | E3 | A test compared the text of an error. | `errors.Is`. The one remaining `err.Error()` check asserts that a password is *absent* from an error, which is the point of that test. |
| 11 | K1, O1, S6 | CI ran neither `-race` nor `govulncheck`. | Both in CI; `govulncheck` also runs every week (`.github/workflows/vulncheck.yml`). The 14 vulnerabilities it found in the standard library, `x/net` and `x/image` are fixed by upgrades (see [Known vulnerabilities](../vulnerabilities.html)). |
| 12 | Q18 | Fake and real camera were not held to the same checks once connected. | Contracts `rcpmodtest.Module` and `rcpmodtest.Online` run on every module, against its fake camera server where it has one. |
| 13 | M9 | Generic file names (`service.go`, `camera.go`, `driver.go`). | Every file renamed after what it holds (`camera_service.go`, `rig_file_store.go`, `module_host.go`, `iris_lever.go`...). |
| 14 | T1, Q1-Q3 | Tests lived in the package they tested and read its internals. | Every test package is external (`xxx_test`). The panel model moved to its own package (`panel/panelmodel`) so its rules are tested through its API. The GUI tests drive the window like a user (find a text, click it, read the window again) through a small UI driver (`internal/gui/ui_driver_test.go`). |
| 15 | T16, K10, K11 | About thirty `time.Sleep` calls in the panel controller, GUI and throttler tests. | They run in `testing/synctest` bubbles on a virtual clock. The waits left are bounded polls (`waitFor`) on real sockets, and one live test that measures the CPU over real time. |
| 16 | C16, T21 | Exported identifiers without doc comments; no testable example. | Every exported identifier is documented. The SDK page layout has `Example` tests that also appear in its documentation. |

The new black-box tests found one bug, now fixed: the status bar went blank after a language
switch (the rebuilt window kept writing to the old status texts). They also show that an imported
configuration file is checked only after the user confirms the import: a file that is not valid
JSON still asks first, then reports the error inline. This is kept as is for now.

## Gaps kept (and why)

| # | Rule | Gap | Reason |
|---|---|---|---|
| A | T1 | `export_test.go` files are in the package they serve. | It is the standard Go way to give external tests a narrow, test-only door (`export_test.go` is compiled only with the tests). Modules are `package main` WASM programs, which an external test package cannot import: their tests stay in `package main`. |
| B | C3 | `core.Camera` has 7 methods. | It is the port every module implements, with many real implementations (the plugin camera, the placeholder, test fakes): rule C4 allows it. Splitting it would make every caller put the pieces back together. |
| C | D20 | `core.Value` is a struct (`Kind` plus one field per kind): a `range` value with a `Choice` set can be represented. | A closed union (interface with a private method) would touch all of the GUI and panel code for little gain: `Value.Check` and `ControlSpec.Validate` keep the invariant at every boundary, and the plugin host validates every value a module sends. |
| D | C6 | `sdk/rcpmod` keeps the registered module in a package variable. | A WASM module exports plain functions (`rcp_call`) that the host calls; they need to reach the module the program registered in `main`. It is set once, before any call. |
