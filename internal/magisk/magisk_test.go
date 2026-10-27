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
}
