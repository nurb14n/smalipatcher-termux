// Package magisk assembles the Magisk module zip that carries the patched
// services.jar. Layout and install script mirror the SmaliPatcherEx module
// (module id, ui_print banner, dalvik-cache/services.jar cleanup).
package magisk

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
)

const (
	ModuleID   = "smalipatcher-bynurb14n"
	ModuleName = "SmaliPatcher by nurb14n"
	Version    = "0.1.0"
)

// updateBinary is the Magisk installer script (update-binary). It prints the
// banner, mounts system, installs services.jar, and removes stale
// dalvik-cache oat artifacts generated from the old services.jar.
const updateBinary = `#!/sbin/sh
ui_print "      SmaliPatcher by nurb14n      "
ui_print "   mock provider + mock permission   "
OUTFD=$2
ZIPFILE=$3
mount /system 2>/dev/null || mount -o rw,remount /system 2>/dev/null
mkdir -p /data/adb/modules/smalipatcher-bynurb14n/system/framework
unzip -o "$ZIPFILE" 'system/*' -d /data/adb/modules/smalipatcher-bynurb14n/
for f in $(find /data/dalvik-cache -iname '*@services.jar*class*' 2>/dev/null); do
  rm -f "$f"
done
for f in $(find /data/misc/apexdata -iname '*@services.jar*class*' 2>/dev/null); do
  rm -f "$f"
done
ui_print "  done."
`

func moduleProp() string {
	return "id=" + ModuleID + "\n" +
		"name=" + ModuleName + "\n" +
		"version=" + Version + "\n" +
		"versionCode=1\n" +
		"author=nurb14n\n" +
		"description=Set mock provider as genuine location provider. Return mock provider has op mock permissions.\n"
}

// Build creates outZip containing the patched services.jar plus Magisk
// metadata. patchedJar is the path to the rebuilt services.jar.
func Build(patchedJar, outZip string) error {
	jar, err := os.ReadFile(patchedJar)
	if err != nil {
		return fmt.Errorf("read patched jar: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(outZip), 0o755); err != nil {
		return err
	}
	f, err := os.Create(outZip)
	if err != nil {
		return err
	}
	defer f.Close()
	w := zip.NewWriter(f)
	defer w.Close()
	add := func(name string, data []byte, executable bool) error {
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		if executable {
			h.SetMode(0o755)
		} else {
			h.SetMode(0o644)
		}
		fw, err := w.CreateHeader(h)
		if err != nil {
			return err
		}
		_, err = fw.Write(data)
		return err
	}
	if err := add("module.prop", []byte(moduleProp()), false); err != nil {
		return err
	}
	if err := add("META-INF/com/google/android/update-binary", []byte(updateBinary), true); err != nil {
		return err
	}
	if err := add("META-INF/com/google/android/updater-script", []byte("#MAGISK\n"), false); err != nil {
		return err
	}
	if err := add("system/framework/services.jar", jar, false); err != nil {
		return err
	}
	return w.Close()
}
