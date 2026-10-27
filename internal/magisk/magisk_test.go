package magisk

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildModule(t *testing.T) {
	dir := t.TempDir()
	jar := filepath.Join(dir, "services.jar")
	if err := os.WriteFile(jar, []byte("fake-jar"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "smalipatcher-bynurb14n.zip")
	if err := Build(jar, out); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.OpenReader(out)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	names := map[string]bool{}
	var prop string
	for _, f := range zr.File {
		names[f.Name] = true
		if f.Name == "module.prop" {
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			b := make([]byte, f.UncompressedSize64)
			n := 0
			for n < len(b) {
				m, err := rc.Read(b[n:])
				n += m
				if err != nil {
					break
				}
			}
			rc.Close()
			prop = string(b)
		}
	}
	for _, want := range []string{
		"module.prop",
		"META-INF/com/google/android/update-binary",
		"META-INF/com/google/android/updater-script",
		"system/framework/services.jar",
	} {
		if !names[want] {
			t.Errorf("missing %s in module zip", want)
		}
	}
	if !strings.Contains(prop, "id="+ModuleID) {
		t.Errorf("module.prop missing id=%s:\n%s", ModuleID, prop)
	}
	if !strings.Contains(prop, "name="+ModuleName) {
		t.Errorf("module.prop missing name=%s:\n%s", ModuleName, prop)
	}
	// Alignment check: services.jar data must start at a 4-byte boundary
	// (zipalign -f 4 equivalent). Walk the central directory to find the
	// entry's local-header offset, then compute the data offset.
	raw, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	eocd := -1
	for i := len(raw) - 22; i >= 0 && i >= len(raw)-22-65557; i-- {
		if raw[i] == 'P' && raw[i+1] == 'K' && raw[i+2] == 5 && raw[i+3] == 6 {
			eocd = i
			break
		}
	}
	if eocd < 0 {
		t.Fatal("end of central directory not found")
	}
	cdOff := int(raw[eocd+16]) | int(raw[eocd+17])<<8 | int(raw[eocd+18])<<16 | int(raw[eocd+19])<<24
	found := false
	for off := cdOff; off+46 <= len(raw); {
		if !(raw[off] == 'P' && raw[off+1] == 'K' && raw[off+2] == 1 && raw[off+3] == 2) {
			break
		}
		method := int(raw[off+10]) | int(raw[off+11])<<8
		nameLen := int(raw[off+28]) | int(raw[off+29])<<8
		extraLen := int(raw[off+30]) | int(raw[off+31])<<8
		commentLen := int(raw[off+32]) | int(raw[off+33])<<8
		name := string(raw[off+46 : off+46+nameLen])
		lho := int(raw[off+42]) | int(raw[off+43])<<8 | int(raw[off+44])<<16 | int(raw[off+45])<<24
		if name == "system/framework/services.jar" {
			found = true
			lNameLen := int(raw[lho+26]) | int(raw[lho+27])<<8
			lExtraLen := int(raw[lho+28]) | int(raw[lho+29])<<8
			dataOff := lho + 30 + lNameLen + lExtraLen
			if method != 0 {
				t.Errorf("services.jar not stored (method=%d), alignment N/A", method)
			}
			if dataOff%4 != 0 {
				t.Errorf("services.jar data offset %d not 4-byte aligned", dataOff)
			}
			break
		}
		off += 46 + nameLen + extraLen + commentLen
	}
	if !found {
		t.Error("services.jar entry not found while checking alignment")
	}
}
