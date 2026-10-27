# smalipatcher-termux

Terminal rewrite (Go, zero dependencies) of the SmaliPatcherEx
**mock provider** + **mock permission** patch flow, generalized to
**Android 11–17 (API 30–37)**. Runs **only on Termux** (Android/aarch64):
it shells out to the Termux packages `adb`, `apktool` and `java`.

Reference: `SmaliPatcherEx-0.1.2.2.zip` (sabpprook, MD5
`853628a96dbe08b5228504655004b2b4`) — a Windows .NET tool that pulls
`services.jar` from the device, decompiles it with apktool 2.9.3, applies
regex text patches to `.smali` files, recompiles, zipaligns, and packs a
Magisk module. The original supports Android 11–14 (decode flags
`d -b -f -k -api`); this rewrite targets apktool 3.x (`d --no-debug-info
-f --keep-broken-res`, no `-api` on decode) and adds variant-tolerant
rules plus `--api` targeting so API 35–37 (Android 15–17) layouts are
handled: unknown files or renamed methods are skipped with a report
instead of failing.

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

## Install (Termux)

```sh
pkg install android-tools   # adb
# java-21, apktool 3.0.3, aapt2: Termux packages (see commit history)
make install                # builds + installs `smalipatcher` to $PREFIX/bin
```

## Usage (on-device)

Aktifkan **Wireless debugging** (Android 11+), `adb pair` +
`adb connect localhost:PORT`, lalu:

```sh
smalipatcher pull -o services.jar
smalipatcher disasm -api 34 services.jar out
smalipatcher patch --mock-provider --mock-permission --api 34 out
smalipatcher verify --all --api 34 out
smalipatcher recomp out dist/services.jar
smalipatcher build dist/services.jar /sdcard/Download/smalipatcher-bynurb14n.zip
```

Lalu flash zip dari `/sdcard/Download` via aplikasi Magisk → reboot.

## Layout

- `cmd/smalipatcher` — CLI (`pull/disasm/patch/verify/recomp/build`)
- `internal/patcher` — rule table + regex engine + verify (`go test ./...`)
- `internal/adb` — device pull / API-level readout
- `internal/magisk` — Magisk module zip builder (built-in 4-byte
  alignment; no zipalign binary exists for aarch64-android)
- `testdata/mock` — pre-patch smali fixtures (golden tests)

## Notes / limits

- API 35–37 layouts are best-effort: run `verify` on a real dump to lock
  in renamed files/methods for a specific ROM.
- Hanya Termux yang didukung (prefix `/data/data/com.termux/files/usr`);
  tidak untuk proot distro / desktop Linux.
