# AGENTS.md — context for any agent working on this repo

Read this first. It says what the project is, what has been done, where everything
is, and the traps already hit. Last updated: 2026-10-18 (0.3.0-ALPHA).

## 1. Project in one paragraph

**RCP For All Camera**: make a Blackmagic **ATEM Micro Camera Panel** (USB) work as a
universal camera control panel on a Windows PC **with no ATEM switcher**. It will
control Blackmagic (Pocket 4K/6K, Studio 4K/6K), Sony (FX3, FX6, FX9, FR7) and
Panasonic (AW-UE100/150/160) cameras over the network. Camera buttons 1–8 will also route the
source to an aux on a Videohub, an ATEM or vMix. The code is open source and modular. Language:
**Go**, built as `rcp.exe` plus one **WebAssembly module per camera model** in a `modules`
folder, with a **native Fyne GUI** (the owner refused a web UI: no Wails, no HTML/WebView).
Owner: Valentin. **He writes in French, so answer in French in the chat; everything else is in
English**: code, comments, strings, docs, reports, commit messages, API.

- Documentation home: [docs/index.html](docs/index.html) (serve it over HTTP to view it:
  `python -m http.server 8777 --directory docs`)
- Intent: [docs/intent/rcp-for-all-camera.md](docs/intent/rcp-for-all-camera.md)
- Spec, capability map and build order: [SPEC.md](SPEC.md) (still DRAFT; open question 1 is resolved)
- Engineering rules every change must follow: [docs/go-engineering-rules.md](docs/go-engineering-rules.md);
  last audit: [docs/reports/2026-10-17-rules-audit.md](docs/reports/2026-10-17-rules-audit.md)

## 2. Status

| Part | State |
|---|---|
| USB panel driver | **Done and verified on hardware** (Windows only): [internal/surfaces/micropanel](internal/surfaces/micropanel) |
| Panel controller | **Wired in the app**: [internal/surfaces/panel](internal/surfaces/panel) (controller) and [panelmodel](internal/surfaces/panel/panelmodel) (pure rules: displays, LEDs, iris lever window, soft takeover). Mapping in [docs/panel-mapping.md](docs/panel-mapping.md). Hardware render check pending owner photo (`go run ./cmd/panelcheck -t 60s`) |
| Panel simulator, panel source | **Done**: on-screen panel ([internal/surfaces/panelsim](internal/surfaces/panelsim), [docs/panelsim.md](docs/panelsim.md)); click the panel status to pick the first USB panel, one by serial, or the simulator ([internal/surfaces/panelsource](internal/surfaces/panelsource)). Multi-panel enumeration untested with two real panels |
| `core`, `config`, `app` | **Done, tested**: contracts, registry, pages, rig persistence through ports (`app.Store`, `app.Secrets` in [persistence_ports.go](internal/app/persistence_ports.go)), settings, events, local presets ([docs/app-api.md](docs/app-api.md)) |
| Module host and SDK | **Done**: [internal/plugins](internal/plugins) loads `modules/*.wasm` with wazero in a sandbox (no file system, no environment, network only to the camera, 1 MiB messages, 64 MiB memory, 10 s per call; a crash stops only that camera). The SDK is [sdk/rcpmod](sdk/rcpmod) with its test kit [rcpmodtest](sdk/rcpmod/rcpmodtest) (`Module` and `Online` contracts, virtual clock). Guide: [docs/modules-camera.html](docs/modules-camera.html) |
| Camera modules | [modules/](modules): **bm-studio-6k-pro** has the real protocol (REST + event WebSocket), read and write verified on hardware. **demo** is simulated (no address, every control answers). The other 14 models are `rcpmod.Planned`: controls and page declared, protocol not written. Each module has a `ui.go` describing its whole camera page |
| `gui` | Native Fyne GUI in [internal/gui](internal/gui), FR/EN via [internal/i18n](internal/i18n) ([docs/i18n.md](docs/i18n.md)). Cameras page (strips), details page laid out from the module's page ([details_page.go](internal/gui/details_page.go), [details_cards.go](internal/gui/details_cards.go)), custom title bar ([titlebar.go](internal/gui/titlebar.go), [window_chrome_windows.go](internal/gui/window_chrome_windows.go)), Settings modal ([settings_modal.go](internal/gui/settings_modal.go)). Ports in `gui.Options`: `Stats`, `Files`, `OpenURL` |
| Routers | **Not started** |

Phase 1 (USB feasibility spike) is complete, verdict **GO**:
[docs/reports/2026-10-11-micro-camera-panel-usb-spike.md](docs/reports/2026-10-11-micro-camera-panel-usb-spike.md).

## 3. Repo map

```
AGENTS.md, SPEC.md, README.md
go.mod                            ← module github.com/valentindoche/rcp-for-all-camera; deps: fyne, wazero
cmd/rcp/                          ← the app: wires everything (flags -sim, -modules)
cmd/buildmodules/                 ← compiles modules/<id>/ to <id>.wasm (internal/modbuild)
cmd/panelcheck, panelprobe, paneltest  ← panel tools and the old test bench
internal/core/                    ← controls, values, events, models, pages, registry, errors (depends on nothing)
internal/app/                     ← Service (use cases) + Host (lifecycle); declares its ports
internal/config/                  ← rig.json store and DPAPI secrets (implements app ports)
internal/plugins/                 ← WASM module host and sandbox; pluginstest builds the real modules for tests
internal/cameras/camerakit, cameratest  ← placeholder camera, control sets, core-side camera contract
internal/gui/                     ← Fyne GUI; widgets/, texts/, style/, icons/, brand/
internal/surfaces/micropanel/     ← USB HID driver (pure Go, no cgo)
internal/surfaces/panel/          ← panel controller; panelmodel/ holds its pure rules
internal/surfaces/panelsim/, panelsource/
internal/sysinfo, autostart, i18n, archtest (dependency rules test)
sdk/rcpmod/                       ← the module SDK; rcpmodtest/ is its test kit
modules/<id>/                     ← one camera model per folder: main.go, ui.go, tests
docs/index.html, modules-camera.html, vulnerabilities.html, assets/  ← HTML documentation
docs/*.md                         ← architecture, app API, adding a camera, protocol, panel mapping, security...
.github/workflows/                ← ci.yml, release.yml, vulncheck.yml (weekly)
```

Never commit build outputs (`*.exe`, `dist/`, `*.wasm`; all git-ignored). Never build modules
into the source `modules/` folder.

## 4. Commands

Fyne needs cgo. gcc is copied to `C:\mingw\mingw64\bin` (first on the user PATH; see trap 14).
An already-open shell may need:

```bash
export PATH="/c/mingw/mingw64/bin:$PATH" CGO_ENABLED=1
go build -ldflags "-H windowsgui" -o dist/rcp.exe ./cmd/rcp   # the app (first build ~4 min)
go run ./cmd/buildmodules                                # every module → dist/modules, next to dist/rcp.exe
go run ./cmd/buildmodules demo                           # only some modules
./dist/rcp.exe -sim                                      # simulator for this run
./dist/rcp.exe -modules <folder>                         # modules from another folder
```

```bash
gofmt -l internal sdk modules cmd   # must print nothing
go vet ./...
go test ./...                       # no hardware, no network; builds the modules once
go test -race ./internal/...
go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...    # known vulnerabilities (see §11)
BMD_CAMERA=<ip> [BMD_CAMERA_WRITE=1] go test ./internal/plugins -run Live -v   # real Studio 6K Pro
RCP_SHOTS_DIR=<folder outside the repo> go test ./internal/gui -run TestScreenshots  # GUI screenshots
go run ./cmd/panelcheck -t 60s      # fake rig on the real panel
go run ./cmd/panelprobe -t 10s      # authenticate + dump input reports as hex
go run ./cmd/paneltest              # old test bench → http://127.0.0.1:8765
```

CI/CD (.github/workflows): `ci.yml` runs gofmt, vet, the tests with `-race` and govulncheck on a
Windows runner for every push to main and every PR. `vulncheck.yml` runs govulncheck every Monday.
Pushing a `v*` tag runs `release.yml`: tests, builds `rcp.exe` and every module, zips them with
the README and publishes a GitHub release (a tag with a suffix such as `v0.3.0-ALPHA` is a
pre-release). The first run of a job compiles Fyne (~7 min).

## 5. Protocol cheat-sheet (details in docs/atem-micro-camera-panel-protocol.md)

- USB VID `0x1EDB`, PID `0xBF01`. HID interface **MI_02**, usage page `0xFF00`. MI_00 is a "Media" interface and is not used.
- **Auth required** before any input arrives: feature `0x05` gives the challenge →
  `05 01` → `05 03` + response, and the panel replies `04` + a 600 s timeout. The
  algorithm is the DaVinci Speed Editor one. Re-auth every 400 s.
- **Inputs**
  - `0x03`: up to 6 held key codes (u16). Cameras 0x40–47, scenes 0x48–4C, STORE 4D,
    ND▲▼ 4E/4F, CC 50/51, GAIN 52/53, SHUTTER 54/55, ON 56, W/B 57, BARS 58, bottom buttons 59–5E (names inferred).
  - `0x0E`: 9 × u16 absolute knob positions 0..1024. Order: WR, BR, WG, BG, WB, BB, top-right knob, SENS, COARSE.
  - `0x0F`: int32 `[1, iris lever, master-black ring]`, ±4096.
  - 0x0E and 0x0F can also be read as feature reports to get the current positions.
- **Outputs**
  - `0x10`: 14 seven-segment digits. Bit 7 = a … bit 1 = g, bit 0 = DP. Order: ND, CC, GAIN×2, SHUTTER×4, IRIS×3, MB×3.
  - `0x11`: matrix 7×10, 2 bits per pixel (1 red, 2 green, 3 amber), p = row*10+col, LSB first.
  - `0x02`: 30-byte LED bitmap + layer byte. Everything is on layer 0, LEDs 0–74. Iris bar 0–15,
    camera RGB 16+3c..18+3c, scene/store 40–47, arrows 48–55, REL/ON/ABS/WB/BARS 56–61,
    D EXT/EXT 62/63, IRIS/MB 64(w)/65(r), AUTO IRIS 66, CALL 67(w)/68(r),
    NETWORK/ALARM/CABLE 69–71, PANEL ACTIVE 72(w)/73(r), PREVIEW 74.

## 6. Traps already hit (don't repeat them)

1. **Feature reports need their exact size.** Padding a `HidD_GetFeature`/`SetFeature`
   buffer to the 64-byte max makes the panel fail with `ERROR_GEN_FAILURE`.
   Output reports (`WriteFile`) are fine padded to 33 bytes.
2. Use **two handles** on the HID path (one blocking reader, one for writes/features),
   because synchronous Windows handles serialize I/O.
3. **Do not set LED bits above 74**: they corrupt the camera matrix.
4. **ATEM Software Control must be closed**: it also authenticates the panel and overwrites the displays.
5. Python on this machine defaults to **cp1252**. Always `open(..., encoding='utf-8', newline='\n')`
   when editing files by script. A cp1252 write once truncated `cmd/paneltest/main.go`.
6. Scripted edits produced CRLF, so run `gofmt -w` after them.
7. **LED layers are persistent in the panel.** A bit left set on layers 1–5 (for
   example by the LED explorer) stays lit forever, even though the program keeps
   rewriting layer 0. `micropanel.Open` clears all 6 layers, and `Panel.ClearLEDs(keep)`
   wipes the others. Bug seen: the iris "11" segment stuck on.
8. Input reports are only sent on change, so read features 0x0E/0x0F at connect
   (already done in `Panel.Run`).
9. The **Bash tool mangles heredocs** containing apostrophes; write files with the
   Write tool, or put a script in a file and run it.
10. The owner wants a **native** GUI. Wails (HTML in WebView2) was built then removed.
11. **Blackmagic REST API**: the camera serves its own OpenAPI docs at
    `http://<ip>/control/documentation.html`, but they are not exact (a copy is in
    `modules/bm-studio-6k-pro/openapi`). Focus is read and written as `normalised`, not `focus`.
    The event socket is `/control/api/v1/event/websocket`, and the subscribe answer carries the
    current values. Numbers come back as float32 (0.1 → 0.10000000149), so the module rounds
    them to 6 decimals. The iris snaps to the lens' stops.
    The camera's own presets (`.cset` files) do **not** save color correction, so presets are
    local only ([internal/app/camera_presets.go](internal/app/camera_presets.go)), and a module
    must never use the camera's `/presets` endpoints.
12. **Iris is in f-stops** (`apertureStop`, unit `"f"`). Presets stored before this change hold a
    normalised iris (0..1): recalling them reports an invalid iris value; store them again.
    The REST API gives no current focus distance, only the lens' range
    (`/lens/focus/description`), assumed in millimetres (`TODO(verify)` in
    `modules/bm-studio-6k-pro/rest_properties.go`).
13. Fyne only recolors SVG fills; Tabler icons are strokes, so `icons.Res` rewrites the
    stroke color. The default font has no "→" glyph.
14. The username has a space (`C:\Users\VALENTIN DOCHE`), which breaks the MinGW linker.
    gcc is copied to `C:\mingw\mingw64\bin`, which is first on the user PATH.
15. **Fyne 2.8 wraps popups** in a `*widget.OverlayContainer`: `Overlays().Top()` is never the
    `*widget.PopUp` itself. GUI tests check for any overlay (`dialogOpen`); the Settings modal is
    not an overlay (trap 18).
16. Fyne's SVG renderer ignores `<rect rx>` (draw rounded rectangles as paths), and draws no
    `<text>` nor `<g transform>`: check docs SVGs in a browser, not with Fyne.
17. The OS title bar is removed by subclassing the window procedure (WM_NCCALCSIZE,
    WM_NCHITTEST). The free area of the custom title bar answers HTCAPTION; its buttons and the
    panel status are "holes" (`captionZones`). The window procedure also takes back the captioned
    frame GLFW sizes for (WM_WINDOWPOSCHANGING, WM_GETMINMAXINFO), otherwise every content change
    grew the window by 16 × 39 px.
18. Do not use a Fyne modal popup for full-window dialogs: it covers the custom title bar (close
    becomes unreachable). Settings is a layer under the title bar (`a.modal`). It covers the
    bottom bar too: the gear opens Settings, only its Close button closes it.
19. The main window must be Fyne's master (`SetMaster`): otherwise a hidden window (the panel
    simulator) keeps the process alive after the main window closes.
20. **The iris lever drives `irisNorm`, not the f-stop, when the camera offers it** (as ATEM
    Software Control does). `normalised` 0 = open, 1 = closed (TODO(verify) live). The Studio 6K
    Pro firmware has no `/lens/iris/description`: its lens iris limits are learned from the stops
    reported at normalised 0 and 1.
21. A camera answers the lever late and snaps or clamps the value: reports within 800 ms of a
    lever write and near the values sent are echoes, not changes from elsewhere
    ([lever_echo.go](internal/surfaces/panel/lever_echo.go)).
22. **GUI tests are black-box** (package `gui_test`): use the driver in
    [ui_driver_test.go](internal/gui/ui_driver_test.go) and the parts in
    [window_parts_test.go](internal/gui/window_parts_test.go). Find things by the text they show,
    click with `press`/`click` (they scroll the target into view first: settings below the fold
    are outside the canvas otherwise). Creating a second `gui.App` on the same test app breaks
    clicks on the first one: do it last in a test.
23. **Timers in tests run on synctest's virtual clock** (`testing/synctest`, helper `settle(d)`
    in the panel tests). A goroutine started outside the bubble cannot be waited for inside it.
24. Modules are `package main` built for `GOOS=wasip1 GOARCH=wasm -buildmode=c-shared`; their
    tests run natively. `pluginstest` builds every module once per test process (a few seconds).

## 7. What was tried and abandoned

- **Middle Control 4.0.4 (macOS DMG)** has no USB code for this panel. It goes through a real
  ATEM switcher ("ATEM Link" + `BMDSwitcherAPI`). Don't re-analyse it.
- **Posing as an ATEM switcher** is unnecessary because the panel is driven directly over HID.
- **Camera drivers compiled into the exe**, then **a shared brand base with hooks** (`bm`): both
  replaced by independent WASM modules (owner decision, 0.3.0-ALPHA). No module shares code
  with another; old model ids (`blackmagic.studio6kpro`...) were dropped without migration.
- The protocol was recovered from live USB captures and static analysis of
  `C:\Program Files (x86)\Blackmagic Design\Blackmagic ATEM Switchers\ATEM Software Control\ATEM Software Control.exe`.
  Panel code addresses (image base 0x140000000) are listed in the spike report.

## 8. Open items (pick up here)

1. Confirm the names of key codes 0x59–0x5E in the `paneltest` log, and check whether the
   square BLACK/FLARE button sends anything (no code and no LED found so far).
2. Run the full 42/42 checklist in `paneltest` with the owner.
3. Confirm on hardware that **writing** feature 0x0E (knob positions) works: the panel controller
   recenters the color knob counters to 512 at connect and near an end (`recenterKnobs`).
4. Meaning of LED layers 1–5 and of LED bits above 74.
5. Untested, from static analysis only: feature 0x06 (battery), 0x09/0x0A (input IDs),
   0x12/0x13 (scene files stored in the panel), 0x15 (name), 0x04, input 0x0D.
6. No non-Windows build of the HID driver yet (`*_windows.go` only).
7. **Owner requirement CB-1** (SPEC "Control Behaviour Requirements"): iris lever and master
   black ring soft takeover. Implemented in `panelmodel` and the controller; verify on hardware.
8. Next: confirm the panel rendering with owner photos (`cmd/panelcheck`), then write the next
   camera protocols as modules (Panasonic AW CGI, Sony), then routers. Every control range marked
   `TODO(verify)` in `modules/*/main.go` must be checked against official docs or hardware.
9. Wire the panel's scene buttons 1–5 and STORE to presets: done (see panel-mapping). Presets
   have numbers only (no names, no timestamps).
10. An imported configuration file is checked only after the user confirms the import (see the
    audit report); revisit if the owner wants it validated first.

## 9. Working rules (from SPEC.md boundaries, the engineering rules and the owner's preferences)

- Follow [docs/go-engineering-rules.md](docs/go-engineering-rules.md). Idiomatic Go, `gofmt`,
  errors wrapped with `%w`, `context.Context` on I/O, external test packages, no bare
  `time.Sleep` in tests, explicit file names (M9), every exported identifier documented.
- **No new runtime dependency without asking.**
- Keep protocol details inside the module. Log device commands at debug level. Survive unplug and reconnect.
- **Ask first** before sending anything that could change recording or output on live gear.
- Hardware verification works best when the owner sends phone photos. Design
  experiments that need few photos, for example one binary-coded pattern per photo
  with a pattern label shown on the SHUTTER display.
- The owner asked for multi-agent work with the model chosen per task: Opus for
  reverse-engineering and architecture, Sonnet for well-specified UI/boilerplate.

## 10. Commits (owner's standing rule)

**Commit after every action, without being asked.** As soon as a piece of work is done
and verified (a fix, a feature, a doc update), commit it before moving on or answering.
Never leave finished work uncommitted. **Do not push** unless the owner asks.

### Message format (Conventional Commits, in English)

```
<type>(<scope>): <summary>

<body: what changed and why, wrapped at 72 columns>

Co-Authored-By: <the attribution line given by the harness>
```

- **summary**: imperative or descriptive, lowercase start, no final period, 72 characters
  at most. It says what the commit brings, e.g.
  `feat(blackmagic): iris in f-stops, lens limits, zoom only with power zoom`.
- **body**: optional for a one-line change, otherwise explain the why and anything a
  future reader would not guess (assumptions, `TODO(verify)`, behaviour changes).
- **footer**: always the Co-Authored-By line. A breaking change to a contract (app API
  JSON, `rig.json`, `core` interfaces, the module ABI) adds `!` after the scope and a
  `BREAKING CHANGE: …` footer.

**Types**

| Type | For |
|---|---|
| `feat` | a new capability for the user or another module |
| `fix` | a bug fix |
| `refactor` | code change with no behaviour change |
| `perf` | faster or lighter, same behaviour |
| `test` | tests only |
| `docs` | documentation only (AGENTS.md, docs/, README, comments-only changes) |
| `build` | go.mod, build flags, embedded assets pipeline |
| `chore` | everything else (tooling, cleanup, renames of non-code files) |

**Scopes**: the module touched, matching the repo layout: `core`, `config`, `app`, `gui`,
`i18n`, `surface` (micropanel driver and panel controller), `plugins`, `sdk`, a module id
(`bm-studio-6k-pro`, `demo`...), `modules` (several modules), `router` (later), `paneltest`,
`panelprobe`. Several modules in one commit: the main one, or none (`docs: …`).

### What makes a commit clean

1. **One logical change per commit.** Split a session's work into several commits
   (for example core, then app, then plugins, then GUI, then docs), in dependency order.
2. **Every commit builds and passes on its own**: `gofmt -l internal sdk modules cmd` empty,
   `go vet ./...`, `go test ./...`. Check a partial commit with
   `git stash push --keep-index --include-untracked`, run the checks, then `git stash pop`.
   If a commit cannot pass alone, move the files it depends on into it.
3. Docs that describe the change go in the same commit or in a `docs:` commit right
   after it.
4. Never commit build outputs, screenshots, scratch files or secrets. Stage files by name,
   not with `git add -A`.
5. Never `--amend` or rewrite a commit that was pushed, never `--no-verify`.

## 11. Known vulnerabilities (check every week)

The list of known vulnerabilities, what was done about each and the test that proves it, is
[docs/vulnerabilities.html](docs/vulnerabilities.html). Keep it current.

**Where to check**

- `go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...` on the repository: it reports only the
  vulnerabilities of the Go standard library and the dependencies that the code actually calls.
- The [Go vulnerability database](https://pkg.go.dev/vuln/) for a given module (fyne, wazero,
  `golang.org/x/*`).
- The GitHub repository's Security tab (Dependabot alerts, private vulnerability reports).
- The [Go release notes](https://go.dev/doc/devel/release) for security fixes of the toolchain.

**Every week** (the `vulncheck.yml` workflow runs govulncheck every Monday at 06:00 UTC and fails
when it finds something; at the start of a session, check its last run or run govulncheck
locally if more than a week has passed):

1. Run govulncheck. If it reports a vulnerability, upgrade the module named in *Fixed in*
   (`go get module@version`, or the `toolchain` line of `go.mod` for the standard library), then
   `go mod tidy`, `go vet ./...`, `go test ./...`.
2. Add a row to the dependencies table of `docs/vulnerabilities.html` (id, where, affected
   version, fixed version, status) and commit (`build: ...` for the upgrade, `docs: ...` for the
   page).
3. For a vulnerability in the application itself (reported privately or found in review): fix
   it, add a test that attacks it, and add a row to the threats table with the test's name.
