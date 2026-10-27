package patcher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		s, d := filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())
		if e.IsDir() {
			copyTree(t, s, d)
			continue
		}
		b, err := os.ReadFile(s)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(d, b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func suitesAll() map[string]bool {
	return map[string]bool{"mock-provider": true, "mock-permission": true, "gnss": true}
}

func TestApplyMockSuites(t *testing.T) {
	dir := t.TempDir()
	copyTree(t, "../../testdata/mock", dir)

	results, err := ApplySuites(dir, map[string]bool{"mock-provider": true, "mock-permission": true}, 37)
	if err != nil {
		t.Fatal(err)
	}
	changed := map[string]bool{}
	for _, r := range results {
		if r.Changed {
			changed[r.Rule.TargetFile+"|"+r.Rule.MethodSig] = true
		}
	}
	want := []string{
		`com/android/server/location/provider/MockLocationProvider.smali|setIsFromMockProvider(`,
		`com/android/server/location/injector/SystemAppOpsHelper.smali|method public noteOp(`,
		`com/android/server/location/injector/SystemAppOpsHelper.smali|method public noteOpNoThrow(`,
	}
	for _, k := range want {
		if !changed[k] {
			t.Errorf("expected change for %s", k)
		}
	}

	// Golden: setIsFromMockProvider call now passes a 0x0 register.
	mlp, _ := os.ReadFile(filepath.Join(dir, "com/android/server/location/provider/MockLocationProvider.smali"))
	if !strings.Contains(string(mlp), "setIsFromMockProvider(Z)V") {
		t.Fatal("setIsFromMockProvider call lost")
	}
	if !strings.Contains(string(mlp), ", 0x0") {
		t.Error("expected a 0x0 register arg after patch")
	}
	// Old true-passing form must be gone.
	if strings.Contains(string(mlp), "{v0, v1}, Landroid/location/Location;->setIsFromMockProvider(Z)V") {
		t.Error("old true-passing call still present")
	}

	// Golden: noteOp bodies forced true.
	helper, _ := os.ReadFile(filepath.Join(dir, "com/android/server/location/injector/SystemAppOpsHelper.smali"))
	if strings.Count(string(helper), "const/4 v0, 0x1") < 2 {
		t.Error("expected both noteOp bodies forced to true")
	}

	// Idempotency: second run changes nothing.
	results2, err := ApplySuites(dir, map[string]bool{"mock-provider": true, "mock-permission": true}, 37)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range results2 {
		if r.Changed {
			t.Errorf("not idempotent: %s %s changed again", r.Rule.TargetFile, r.Rule.MethodSig)
		}
	}

	// Verify passes on patched tree.
	vres, err := Verify(dir, map[string]bool{"mock-provider": true, "mock-permission": true}, 37)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range vres {
		if r.Skipped {
			continue
		}
		if !r.Changed {
			t.Errorf("verify failed: %s %s: %s", r.Rule.TargetFile, r.Rule.MethodSig, r.Detail)
		}
	}

	// Verify fails on pristine tree.
	dir2 := t.TempDir()
	copyTree(t, "../../testdata/mock", dir2)
	vres2, _ := Verify(dir2, map[string]bool{"mock-provider": true, "mock-permission": true}, 37)
	seenFail := false
	for _, r := range vres2 {
		if r.Skipped {
			continue
		}
		if !r.Changed {
			seenFail = true
		}
	}
	if !seenFail {
		t.Error("expected verify to fail on unpatched tree")
	}
}
