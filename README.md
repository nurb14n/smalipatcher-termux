# smalipatcher-app

Terminal rewrite (Go, zero dependencies) of the SmaliPatcherEx
**mock provider** + **mock permission** patch flow, generalized to
**Android 11–17 (API 30–37)**.

Reference: `SmaliPatcherEx-0.1.2.2.zip` (sabpprook, MD5
`853628a96dbe08b5228504655004b2b4`) — a Windows .NET tool that pulls
`services.jar` from the device, decompiles it with apktool, applies regex
text patches to `.smali` files, recompiles, zipaligns, and packs a Magisk
module. The original supports Android 11–14; this rewrite keeps the same
patch semantics and adds variant-tolerant rules plus `--api` targeting so
API 35–37 (Android 15–17) layouts are handled: unknown files or renamed
methods are skipped with a report instead of failing.

## What the patches do

**Mock provider** — test locations are accepted as genuine:
- `.../location/provider/MockLocationProvider.smali` (and the legacy
  `.../location/MockProvider.smali`): the `setIsFromMockProvider(Z)V`
  call argument is rewritten from the `0x1` register to a fresh `0x0`
  register (register window grown by 2, `.local` untouched).
- `LocationManagerService.smali`: `canCallerAccessMockLocation(` and
  `checkMockLocationAccess(` are forced to return true;
  `noteMockLocationAccess(` is stubbed to return void.

**Mock permission** — test providers work without mock permission UX:
- `.../location/injector/SystemAppOpsHelper.smali` (and
  `.../location/AppOpsHelper.smali`): `noteOp(` and `noteOpNoThrow(`
  are forced to return true (`const/4 v0, 0x1; return v0`).

**GNSS (bonus)**: `reportLocation(` in both `GnssLocationProvider`
variants is stubbed to `return-void`.

## Usage

```sh
# build
go build -o smalipatcher ./cmd/smalipatcher

# pull framework from device
./smalipatcher pull -o services.jar

# decompile (needs apktool + java; -api 0 = auto)
./smalipatcher disasm -api 35 services.jar out

# patch (default: all suites; --api 0 = try all known variants)
./smalipatcher patch --mock-provider --mock-permission --api 37 out

# verify post-state (fails only on applicable-but-unsatisfied rules)
./smalipatcher verify --all --api 37 out

# pack Magisk module
./smalipatcher build services.patched.jar smalipatcher-app.zip
```

## Layout

- `cmd/smalipatcher` — CLI (`pull/disasm/patch/verify/build`)
- `internal/patcher` — rule table + regex engine + verify (`go test ./...`)
- `internal/adb` — device pull / API-level readout
- `internal/magisk` — Magisk module zip builder
- `testdata/mock` — pre-patch smali fixtures (golden tests)

## Notes / limits

- API 35–37 layouts are best-effort: run `verify` on a real dump to lock
  in renamed files/methods for a specific ROM.
- `disasm`/`build` shell out to `apktool`/`java`/`zipalign` (same external
  tools as the reference); `patch`/`verify` are pure Go.
