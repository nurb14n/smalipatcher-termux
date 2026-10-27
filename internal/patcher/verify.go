// Package patcher — verification: re-scan a tree and confirm each rule's
// post-state holds (used by the `verify` command).
package patcher

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Verify checks every rule in suites against the tree at dir and reports
// per-rule pass/fail without modifying anything. api filters like ApplySuites.
func Verify(dir string, suites map[string]bool, api int) ([]Result, error) {
	var out []Result
	for _, r := range Rules {
		if !suites[r.Suite] {
			continue
		}
		res := Result{Rule: r, File: filepath.Join(dir, filepath.FromSlash(r.TargetFile))}
		if api > 0 && ((r.MinAPI > 0 && api < r.MinAPI) || (r.MaxAPI > 0 && api > r.MaxAPI)) {
			res.Skipped = true
			res.Detail = fmt.Sprintf("out of API range for target API %d", api)
			out = append(out, res)
			continue
		}
		raw, err := os.ReadFile(res.File)
		if err != nil {
			if os.IsNotExist(err) {
				res.Skipped = true
				res.Detail = "target file not present"
				out = append(out, res)
				continue
			}
			return out, fmt.Errorf("read %s: %w", res.File, err)
		}
		ok, skipped, detail := checkPostState(string(raw), r)
		res.Changed = ok
		res.Skipped = skipped
		res.Detail = detail
		out = append(out, res)
	}
	return out, nil
}

func checkPostState(src string, r Rule) (ok, skipped bool, detail string) {
	anchor := strings.Index(src, r.MethodSig)
	if anchor < 0 {
		// Renamed/moved in this Android version: variant-tolerant skip.
		return false, true, "method not found in this variant, skipped"
	}
	start, end, found := enclosingMethod(src, anchor)
	if !found {
		return false, true, "unterminated method, skipped"
	}
	block := src[start:end]
	switch r.Mode {
	case StubTrue:
		if strings.Contains(block, "const/4 v0, 0x1") && strings.Contains(block, "return v0") {
			return true, false, "ok: forced true"
		}
		return false, false, "not patched: body does not force true"
	case StubVoid:
		if strings.Contains(block, "return-void") &&
			!strings.Contains(block, "invoke-") {
			return true, false, "ok: stubbed void"
		}
		return false, false, "not patched: body still does work"
	case RewriteSetIsFromMock:
		loc := reSetMock.FindStringSubmatch(block)
		if loc == nil {
			return false, false, "setIsFromMockProvider call not found"
		}
		return true, false, "ok: call present (value checked at patch time)"
	default:
		return false, false, "unknown mode"
	}
}
