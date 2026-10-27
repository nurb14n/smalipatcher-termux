#!/data/data/com.termux/files/usr/bin/bash
# One-line installer for smalipatcher-termux (Termux on Android/aarch64):
#   curl -fsSL https://raw.githubusercontent.com/nurb14n/smalipatcher-termux/master/install.sh | bash
set -euo pipefail

REPO="nurb14n/smalipatcher-termux"
BIN="smalipatcher-android-arm64"
PREFIX="${PREFIX:-/data/data/com.termux/files/usr}"

if [ ! -d "$PREFIX" ] || [ "$(uname -o 2>/dev/null)" != "Android" ]; then
  echo "error: this installer only supports Termux on Android" >&2
  exit 1
fi
if [ "$(uname -m)" != "aarch64" ]; then
  echo "error: only Android/aarch64 is supported (detected: $(uname -m))" >&2
  exit 1
fi
if [ "$(id -u)" = "0" ]; then
  echo "error: run as the Termux user, not root" >&2
  exit 1
fi

echo "==> installing Termux packages (adb, apktool, java)..."
pkg install -y android-tools apktool aapt2 openjdk-21

echo "==> downloading smalipatcher..."
URL="https://github.com/${REPO}/releases/latest/download/${BIN}"
curl -fsSL -o "/tmp/${BIN}" "$URL"
if curl -fsSL -o "/tmp/${BIN}.sha256" "https://github.com/${REPO}/releases/latest/download/${BIN}.sha256"; then
  (cd /tmp && sha256sum -c "${BIN}.sha256")
else
  echo "warn: no checksum file, skipping verification"
fi

echo "==> installing to ${PREFIX}/bin/smalipatcher ..."
install -m755 "/tmp/${BIN}" "${PREFIX}/bin/smalipatcher"
rm -f "/tmp/${BIN}" "/tmp/${BIN}.sha256"

echo "==> verifying..."
command -v smalipatcher
smalipatcher 2>&1 | head -n 4

cat <<'EOF'

Done. Next steps (same device):
  1. Enable Wireless debugging (Developer options, Android 11+)
  2. adb pair localhost:PAIR_PORT   # once
     adb connect localhost:PORT
  3. smalipatcher auto --api $(getprop ro.build.version.sdk 2>/dev/null || echo 34)
     # or step by step: pull -> disasm -> patch -> verify -> recomp -> build
  4. Flash /sdcard/Download/smalipatcher-bynurb14n.zip in Magisk, reboot.
EOF
