# Security

Scope: the Go backend (`internal/core`, `internal/config`, `internal/app`, `internal/cameras`,
`internal/surfaces/micropanel`). Last reviewed 2026-10-11.

## Threat model

RCP For All Camera is a local Windows desktop app. It opens **no network listener**. It
talks to a USB HID panel and, later, to cameras on the production LAN.

| Boundary | Untrusted input | Main risks |
|---|---|---|
| GUI → `app.Service` | `SlotInput`, slot numbers, control ids, `core.Value` | Malformed or huge strings in logs, errors, camera commands. Spoofed names (bidi, zero-width) |
| Disk → `config` | `rig.json`, `secrets.dat` | Corrupt or hostile file. Memory exhaustion. Password disclosure |
| USB → `micropanel` | Input and feature reports | Out-of-bounds reads, panics from a faulty or malicious device |
| LAN → camera drivers (future) | Camera responses, DNS, anything on the network path | Credential theft, spoofed camera, oversized or slow responses |

**Assets:** camera passwords; control of live production gear (recording, output, tally).

**Out of scope:** malware running as the same Windows user, and local administrators.
Both can read process memory and call DPAPI as the user. No local app can defend
against them.

## What is protected, and how

**Camera passwords**
- They never go into `rig.json`: `CameraConfig.Password` is `json:"-"`.
- They are stored in `%APPDATA%\RCPForAllCamera\secrets.dat`, encrypted with Windows
  DPAPI (`CryptProtectData`, current-user scope, UI forbidden). The DPAPI call uses
  app-specific optional entropy, so a bare `CryptUnprotectData` from another program
  fails. The entropy is a namespace, not a secret.
- The file has a magic header, a 64 KiB read limit and an atomic write (temp file,
  then rename). There is no plaintext cache: each call decrypts the file, and the
  decrypted buffers (OS buffer and Go copy) are wiped after use. Go strings handed to
  the app cannot be wiped. This is a known limit.
- A file that cannot be decrypted (corrupt, tampered, from another user or PC) returns
  `config.ErrSecretsUnreadable` and is **never overwritten automatically**.
- Passwords are never returned to the GUI. `CameraView.HasPassword` only says whether
  one is stored.

**Logging.** `core.CameraConfig` and `app.SlotInput` implement `String`, `GoString`
and `slog.LogValuer` with the password redacted, so `%v`, `%+v`, `%#v` and slog
attributes are safe. `core.Status.Detail` must not contain credentials.

**Input validation** at the `app.Service` boundary and in `core`:
- Slots: 1..8. `SelectSlot` also accepts 0.
- Model id: at most 64 bytes. Longer ids are rejected before lookup.
- `ValidateConfig` checks:
  - name ≤ 64 bytes;
  - username and password ≤ 128 bytes;
  - all three valid UTF-8 with no control or format characters (C0/C1, bidi
    overrides, zero-width);
  - host is an IP literal or an RFC 1123 name (no URL, path, port or space);
  - port 0..65535;
  - transport is declared by the model.
- Control id: 1..64 bytes of `[A-Za-z0-9._-]`. Enum choice: ≤ 64 printable bytes. The
  value kind must be known. These errors never echo the input.
- `ControlSpec.Validate` checks range, NaN and enum membership before any I/O.

**Errors to the GUI.** Errors are mapped to stable codes. Unknown errors become
`INTERNAL` with a generic message, so paths, OS errors and DPAPI details stay in the
logs.

**Config file.** Read with a 1 MiB limit and version check. Entries are re-validated
against the model registry at startup, and invalid ones are dropped. An unusable file
is renamed to `rig.json.bad` instead of being overwritten by the next save. Unknown
JSON fields are ignored for forward compatibility.

**USB panel.** Device input is untrusted. `parseReport` length-checks every report and
is covered by a native fuzz test (`FuzzParseReport`). Report buffers are bounded to
1..64 bytes, and the device-interface detail buffer is size-checked before it is
written.

**Robustness.** Each GUI call is bounded by a single 3 s deadline (`Controls` uses one
deadline for all its reads). If saving the rig fails, `AssignSlot` restores the
previous secret, so `secrets.dat` matches `rig.json`.

## Residual risks

- **File permissions on Windows.** Go ignores the `0600`/`0700` modes on Windows.
  `rig.json` and `secrets.dat` inherit the ACL of `%APPDATA%`, which by default grants
  access to the user, SYSTEM and Administrators only. We do not set an explicit ACL:
  `rig.json` holds no secret, and `secrets.dat` is protected by DPAPI regardless of
  its ACL.
- **Same-user code.** Same-user code can decrypt `secrets.dat`. Passwords also stay
  in process memory while the app runs.
- **Roaming.** `%APPDATA%` roams, and DPAPI user keys roam with the profile. On a
  machine without the roamed keys, the file is unreadable. The app must then ask for
  the passwords again.
- **Host names.** A host name is resolved by DNS at connect time. LAN DNS spoofing can
  redirect a camera connection. Use IP literals on production networks.
- **Toolchain.** Go 1.27.0 has stdlib advisories fixed in 1.27.2 (see govulncheck
  output). None is reachable from this code today. Upgrade the toolchain before release.

## Rules for camera drivers

1. Use TLS when the camera offers it, and digest or token auth rather than basic auth
   over cleartext HTTP. If a camera only supports cleartext, say so in `ModelInfo.Notes`.
2. Every network call takes the caller's `context.Context`. Also set explicit dial,
   TLS handshake and response-header timeouts. Never use `http.DefaultClient`.
3. Bound everything you read: `io.LimitReader` on bodies (for example 1 MiB),
   WebSocket read limits, a maximum frame and line length for VISCA and other binary
   protocols, and a cap on reconnect backoff.
4. Treat camera responses as untrusted. Validate them before use: numbers within the
   spec range, enum values in the option list, strings length-checked and stripped of
   control characters before they reach the GUI or the panel displays.
5. Never log credentials, `Authorization` headers or URLs with userinfo. Set
   credentials with `req.SetBasicAuth` or a digest helper, never in the URL. Keep
   `Status.Detail` and error text free of secrets. Log device commands at debug level
   only.
6. Build addresses with `net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))`, never
   by string concatenation. Do not follow HTTP redirects to another host.
7. **Ask the owner before sending anything that can change recording or output on
   live gear** (record, tally, colour bars, presets), including in tests. Contract
   tests run offline.
8. No new third-party dependency without asking. Run `govulncheck` before each release.

## Wiring the secret store (`cmd/rcp`)

```go
dir := filepath.Dir(rigPath)
secrets, err := config.NewOSSecrets(dir)
if err != nil {
	// ErrNoOSSecrets (non-Windows) or ErrSecretsUnreadable (corrupt or foreign file).
	slog.Warn("secret store unavailable, passwords kept in memory only", "err", err)
	secrets = config.NewMemorySecrets()
	// Tell the user: passwords will have to be re-entered after restart.
}
```
