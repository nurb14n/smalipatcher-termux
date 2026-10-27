// Package patcher implements the smali rewrite rules reverse-engineered
// from SmaliPatcherEx 0.1.2.2 (sabpprook, .NET assembly, verified by
// MD5 853628a96dbe08b5228504655004b2b4 and .NET metadata + #US strings).
//
// Two patch suites are implemented:
//
//	Mock provider  (Patch.mockProvider in the original):
//	  - com/android/server/location/MockProvider.smali
//	  - com/android/server/location/provider/MockLocationProvider.smali
//	  Target methods: setIsFromMockProvider(, canCallerAccessMockLocation(,
//	  noteMockLocationAccess(, checkMockLocationAccess(
//	  Effect: "Set mock provider as a genuine location provider."
//
//	Mock permission (Patch.mockPermission in the original):
//	  - com/android/server/location/injector/SystemAppOpsHelper.smali
//	  Target methods: noteOp(, noteOpNoThrow(
//	  Effect: "Return mock provider has op mock permissions."
//
// The engine is regex/text based and idempotent: applying it twice
// yields the same result and reports "already patched".
package patcher

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Mode selects how a matched method body is rewritten.
type Mode int

const (
	// StubTrue replaces the body with: const/4 v0, 0x1; return v0
	// (used for boolean gates: canCallerAccessMockLocation,
	// checkMockLocationAccess, noteOp, noteOpNoThrow).
	StubTrue Mode = iota
	// StubVoid replaces the body with: return-void
	// (used for void hooks: noteMockLocationAccess, reportLocation*).
	StubVoid
	// RewriteSetIsFromMock rewrites the setIsFromMockProvider(Z)V call
	// argument from the 0x1 register to a fresh 0x0 register, so test
	// locations are no longer flagged as mock.
	RewriteSetIsFromMock
)

// Rule is one method rewrite inside one smali file.
type Rule struct {
	Suite      string // "mock-provider" | "mock-permission" | "gnss"
	TargetFile string // path relative to the baksmali output dir
	MethodSig  string // anchor inside the target method, e.g. "setIsFromMockProvider("
	Mode       Mode
	Note       string // "Patched: ..." marker, mirrors original log lines
	// MinAPI/MaxAPI document the Android API range the rule is known to
	// cover. 0 means unbounded. The original tool covers API 30-34
	// (Android 11-14); this rewrite is variant-tolerant by design so
	// rules also apply to API 35-37 (Android 15-17): unknown files or
	// renamed methods are skipped with a report instead of failing.
	// Confirm new layouts with the `verify` command on a real dump.
	MinAPI int
	MaxAPI int
}

// Rules is the full rule table. Order matters for the report.
var Rules = []Rule{
	{
		Suite:      "mock-provider",
		TargetFile: `com/android/server/location/provider/MockLocationProvider.smali`,
		MethodSig:  "setIsFromMockProvider(",
		Mode:       RewriteSetIsFromMock,
		Note:       "Patched: setIsFromMockProvider",
	},
	{
		Suite:      "mock-provider",
		TargetFile: `com/android/server/location/MockProvider.smali`,
		MethodSig:  "setIsFromMockProvider(",
		Mode:       RewriteSetIsFromMock,
		Note:       "Patched: setIsFromMockProvider",
	},
	{
		Suite:      "mock-provider",
		TargetFile: `com/android/server/LocationManagerService.smali`,
		MethodSig:  "method private canCallerAccessMockLocation(",
		Mode:       StubTrue,
		Note:       "Patched: canCallerAccessMockLocation",
	},
	{
		Suite:      "mock-provider",
		TargetFile: `com/android/server/location/LocationManagerService.smali`,
		MethodSig:  "method private canCallerAccessMockLocation(",
		Mode:       StubTrue,
		Note:       "Patched: canCallerAccessMockLocation",
	},
	{
		Suite:      "mock-provider",
		TargetFile: `com/android/server/LocationManagerService.smali`,
		MethodSig:  "method public checkMockLocationAccess(",
		Mode:       StubTrue,
		Note:       "Patched: checkMockLocationAccess",
	},
	{
		Suite:      "mock-provider",
		TargetFile: `com/android/server/location/LocationManagerService.smali`,
		MethodSig:  "method public checkMockLocationAccess(",
		Mode:       StubTrue,
		Note:       "Patched: checkMockLocationAccess",
	},
	{
		Suite:      "mock-provider",
		TargetFile: `com/android/server/LocationManagerService.smali`,
		MethodSig:  "method public noteMockLocationAccess(",
		Mode:       StubVoid,
		Note:       "Patched: noteMockLocationAccess",
	},
	{
		Suite:      "mock-provider",
		TargetFile: `com/android/server/location/LocationManagerService.smali`,
		MethodSig:  "method public noteMockLocationAccess(",
		Mode:       StubVoid,
		Note:       "Patched: noteMockLocationAccess",
	},
	{
		Suite:      "mock-permission",
		TargetFile: `com/android/server/location/injector/SystemAppOpsHelper.smali`,
		MethodSig:  "method public noteOp(",
		Mode:       StubTrue,
		Note:       "Patched: noteOp",
	},
	{
		Suite:      "mock-permission",
		TargetFile: `com/android/server/location/injector/SystemAppOpsHelper.smali`,
		MethodSig:  "method public noteOpNoThrow(",
		Mode:       StubTrue,
		Note:       "Patched: noteOpNoThrow",
	},
	{
		Suite:      "mock-permission",
		TargetFile: `com/android/server/location/AppOpsHelper.smali`,
		MethodSig:  "method public noteOp(",
		Mode:       StubTrue,
		Note:       "Patched: noteOp",
	},
	{
		Suite:      "mock-permission",
		TargetFile: `com/android/server/location/injector/AppOpsHelper.smali`,
		MethodSig:  "method public noteOp(",
		Mode:       StubTrue,
		Note:       "Patched: noteOp",
	},
	// Bonus suite, templates also present in the original binary
	// ("Patched: reportLocation", "Disable GNSS location report.").
	{
		Suite:      "gnss",
		TargetFile: `com/android/server/location/GnssLocationProvider.smali`,
		MethodSig:  "method private reportLocation(",
		Mode:       StubVoid,
		Note:       "Patched: reportLocation",
	},
	{
		Suite:      "gnss",
		TargetFile: `com/android/server/location/gnss/GnssLocationProvider.smali`,
		MethodSig:  "method private reportLocation(",
		Mode:       StubVoid,
		Note:       "Patched: reportLocation",
	},
}

var (
	reRegisters = regexp.MustCompile(`(?m)^(\s*)\.(registers|locals)\s+(\d+)\s*$`)
	reEndMethod = regexp.MustCompile(`(?m)^\s*\.end method\s*$`)
	reSetMock   = regexp.MustCompile(`invoke-virtual\s+\{(\w+),\s*(\w+)\},\s*Landroid/location/Location;->setIsFromMockProvider\(Z\)V`)
)

func init() {
	// All rules derive from the Android 11+ (API 30+) location stack.
	for i := range Rules {
		Rules[i].MinAPI = 30
	}
}

// Result describes what happened for one rule in one file.
type Result struct {
	Rule    Rule
	File    string
	Changed bool
	Detail  string
	// Skipped marks variant-tolerant skips (file absent, API out of
	// range, or method renamed in this Android version). Skips never
	// fail verification; only applicable-but-unsatisfied rules do.
	Skipped bool
}

// ApplySuites applies all rules whose Suite is in suites to the baksmali
// output tree rooted at dir. api is the target Android API level (0 =
// unknown: all variant-tolerant rules are attempted, missing files or
// methods are skipped with a report). Files are edited in place; a ".bak"
// copy is kept next to each modified file on first change.
func ApplySuites(dir string, suites map[string]bool, api int) ([]Result, error) {
	var out []Result
	for _, r := range Rules {
		if !suites[r.Suite] {
			continue
		}
		if api > 0 && ((r.MinAPI > 0 && api < r.MinAPI) || (r.MaxAPI > 0 && api > r.MaxAPI)) {
			out = append(out, Result{Rule: r, File: filepath.Join(dir, filepath.FromSlash(r.TargetFile)), Skipped: true, Detail: fmt.Sprintf("out of API range for target API %d, skipped", api)})
			continue
		}
		res, err := applyRule(dir, r)
		if err != nil {
			return out, err
		}
		out = append(out, res)
	}
	return out, nil
}

// targetFiles resolves a rule target against a baksmali output tree.
// Single-dex trees keep classes directly under dir; multidex trees split
// them across smali/, smali_classes2/, ... All matches are returned so
// every dex slice gets patched.
func targetFiles(dir, rel string) []string {
	rel = filepath.FromSlash(rel)
	seen := map[string]bool{}
	var out []string
	add := func(p string) {
		if seen[p] {
			return
		}
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			seen[p] = true
			out = append(out, p)
		}
	}
	add(filepath.Join(dir, rel))
	matches, _ := filepath.Glob(filepath.Join(dir, "smali*"))
	for _, sm := range matches {
		if st, err := os.Stat(sm); err != nil || !st.IsDir() {
			continue
		}
		add(filepath.Join(sm, rel))
	}
	return out
}

func applyRule(dir string, r Rule) (Result, error) {
	files := targetFiles(dir, r.TargetFile)
	if len(files) == 0 {
		return Result{Rule: r, File: filepath.Join(dir, filepath.FromSlash(r.TargetFile)), Skipped: true, Detail: "target file not present, skipped"}, nil
	}
	// Patch every dex slice that contains the target.
	last := Result{}
	for _, f := range files {
		res, err := applyFile(f, r)
		if err != nil {
			return res, err
		}
		if res.Changed {
			last = res
		} else if last.File == "" {
			last = res
		}
	}
	if len(files) > 1 {
		last.Detail += fmt.Sprintf(" (%d dex slices)", len(files))
	}
	return last, nil
}

func applyFile(file string, r Rule) (Result, error) {
	res := Result{Rule: r, File: file}
	raw, err := os.ReadFile(res.File)
	if err != nil {
		if os.IsNotExist(err) {
			res.Skipped = true
			res.Detail = "target file not present, skipped"
			return res, nil
		}
		return res, fmt.Errorf("read %s: %w", res.File, err)
	}
	orig := string(raw)
	patched, changed, detail := rewriteMethod(orig, r)
	if !changed {
		res.Detail = detail
		return res, nil
	}
	bak := res.File + ".bak"
	if _, err := os.Stat(bak); os.IsNotExist(err) {
		if err := os.WriteFile(bak, raw, 0o644); err != nil {
			return res, fmt.Errorf("backup %s: %w", bak, err)
		}
	}
	if err := os.WriteFile(res.File, []byte(patched), 0o644); err != nil {
		return res, fmt.Errorf("write %s: %w", res.File, err)
	}
	res.Changed = true
	res.Detail = detail
	return res, nil
}

// enclosingMethod expands anchor (an index inside a method, possibly
// mid-line) to the full [.method, .end method] span.
func enclosingMethod(src string, anchor int) (start, end int, ok bool) {
	lineStart := strings.LastIndex(src[:anchor], "\n") + 1
	for {
		line := src[lineStart:]
		if eol := strings.Index(line, "\n"); eol >= 0 {
			line = line[:eol]
		}
		if strings.HasPrefix(strings.TrimSpace(line), ".method ") {
			start = lineStart
			break
		}
		if lineStart == 0 {
			return 0, 0, false
		}
		lineStart = strings.LastIndex(src[:lineStart-1], "\n") + 1
	}
	m := reEndMethod.FindStringIndex(src[anchor:])
	if m == nil {
		return 0, 0, false
	}
	return start, anchor + m[1], true
}

// rewriteMethod locates the enclosing method around the anchor signature
// and rewrites its body.
func rewriteMethod(src string, r Rule) (string, bool, string) {
	anchor := strings.Index(src, r.MethodSig)
	if anchor < 0 {
		return src, false, "method not found, skipped"
	}
	start, end, ok := enclosingMethod(src, anchor)
	if !ok {
		return src, false, "enclosing method not found, skipped"
	}
	block := src[start:end]
	var replacement string
	switch r.Mode {
	case StubTrue, StubVoid:
		// Never inject code into abstract/native declarations (e.g.
		// AppOpsHelper's abstract noteOp on some ROMs) — that would
		// produce invalid dex and risk a bootloop.
		decl := strings.TrimSpace(strings.SplitN(block, "\n", 2)[0])
		if strings.Contains(decl, " abstract ") || strings.Contains(decl, " native ") {
			return src, false, "abstract/native declaration, skipped"
		}
		if r.Mode == StubTrue {
			replacement = stubBody(block, "\n    const/4 v0, 0x1\n\n    return v0\n")
		} else {
			replacement = stubBody(block, "\n    return-void\n")
		}
	case RewriteSetIsFromMock:
		return rewriteSetIsFromMock(src, start, block, end-start)
	default:
		return src, false, "unknown mode"
	}
	if replacement == "" {
		return src, false, "already patched"
	}
	return src[:start] + replacement + src[end:], true, r.Note
}

// stubBody keeps the .registers/.locals + annotation lines, then swaps the
// executable body. Returns "" when the body already matches (idempotent).
func stubBody(block, body string) string {
	lines := strings.Split(block, "\n")
	keep := make([]string, 0, len(lines))
	i := 0
	for ; i < len(lines); i++ {
		keep = append(keep, lines[i])
		if reRegisters.MatchString(lines[i]) {
			break
		}
	}
	rest := strings.Join(lines[i+1:], "\n")
	// Strip trailing ".end method" line for comparison.
	restBody := strings.TrimSuffix(strings.TrimSpace(rest), ".end method")
	restBody = strings.TrimSpace(strings.TrimSuffix(restBody, ".end method"))
	want := strings.TrimSpace(body)
	if restBody == want {
		return ""
	}
	return strings.Join(keep, "\n") + body + ".end method"
}

// rewriteSetIsFromMock implements the core mock-provider trick: the
// Location copy gets setIsFromMockProvider(false) instead of true.
func rewriteSetIsFromMock(src string, start int, block string, tailLen int) (string, bool, string) {
	loc := reSetMock.FindStringSubmatchIndex(block)
	if loc == nil {
		return src, false, "setIsFromMockProvider call not found, skipped"
	}
	// If the call already passes a 0x0 constant, nothing to do. We check
	// the nearest preceding const/4 for the second register.
	callLineStart := strings.LastIndex(block[:loc[0]], "\n") + 1
	_ = callLineStart
	argReg := block[loc[4]:loc[5]]
	pre := block[:loc[0]]
	reConst := regexp.MustCompile(`const/4\s+` + regexp.QuoteMeta(argReg) + `,\s*0x([0-9a-fA-F])\b`)
	if cm := reConst.FindStringSubmatch(pre); cm != nil && strings.EqualFold(cm[1], "0") {
		// Already passing false; make sure no stale const/4 vX, 0x1 feeds it.
		return src, false, "already patched"
	}
	// Grow the register window by 2 and use a fresh top register for 0x0.
	rm := reRegisters.FindStringSubmatchIndex(block)
	if rm == nil {
		return src, false, "no .registers/.locals line, skipped"
	}
	var n int
	fmt.Sscanf(block[rm[6]:rm[7]], "%d", &n)
	newRegs := n + 2
	zeroReg := fmt.Sprintf("v%d", n)
	newBlock := block[:rm[0]] + reRegisters.ReplaceAllStringFunc(block[rm[0]:rm[1]], func(string) string {
		indent := reRegisters.FindStringSubmatch(block[rm[0]:rm[1]])[1]
		kw := reRegisters.FindStringSubmatch(block[rm[0]:rm[1]])[2]
		return fmt.Sprintf("%s.%s %d", indent, kw, newRegs)
	}) + block[rm[1]:]
	// Insert the const/4 zero just before the invoke line, and rewrite arg.
	loc2 := reSetMock.FindStringSubmatchIndex(newBlock)
	invokeStart := strings.LastIndex(newBlock[:loc2[0]], "\n") + 1
	insert := "    const/4 " + zeroReg + ", 0x0\n"
	newBlock = newBlock[:invokeStart] + insert + newBlock[invokeStart:]
	loc3 := reSetMock.FindStringSubmatchIndex(newBlock)
	oldCall := newBlock[loc3[0]:loc3[1]]
	newCall := strings.Replace(oldCall, argReg, zeroReg, 1)
	newBlock = newBlock[:loc3[0]] + newCall + newBlock[loc3[1]:]
	return src[:start] + newBlock + src[start+tailLen:], true, "Patched: setIsFromMockProvider"
}
