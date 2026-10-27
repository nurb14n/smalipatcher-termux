// Command smalipatcher is a terminal rewrite of the SmaliPatcherEx
// mock-provider / mock-permission patch flow, built to run ONLY on Termux
// (Android/aarch64): it shells out to the Termux packages adb, apktool and
// java, and defaults to Termux prefix paths.
//
//	smalipatcher pull    [-s serial] [-o services.jar]
//	smalipatcher disasm  [-jar apktool.jar] out-dir services.jar (apktool 3.x flags)
//	smalipatcher patch   [--mock-provider] [--mock-permission] [--gnss] [--all] [--api N] out-dir
//	smalipatcher verify  [--mock-provider] [--mock-permission] [--gnss] [--all] [--api N] out-dir
//	smalipatcher recomp  out-dir dist/services.jar
//	smalipatcher build   dist/services.jar module.zip
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/nurb14n/smalipatcher-termux/internal/adb"
	"github.com/nurb14n/smalipatcher-termux/internal/magisk"
	"github.com/nurb14n/smalipatcher-termux/internal/patcher"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "pull":
		err = cmdPull(os.Args[2:])
	case "disasm":
		err = cmdDisasm(os.Args[2:])
	case "patch":
		err = cmdPatch(os.Args[2:])
	case "verify":
		err = cmdVerify(os.Args[2:])
	case "recomp":
		err = cmdRecomp(os.Args[2:])
	case "build":
		err = cmdBuild(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`usage: smalipatcher <pull|disasm|patch|verify|recomp|build> [flags]  (Termux-only)

  pull    [-s serial] [-o services.jar]   pull services.jar from device
  disasm  [-jar apktool.jar] [-api N] services.jar out-dir
  patch   [--mock-provider] [--mock-permission] [--gnss] [--all] [--api N] out-dir
  verify  [--mock-provider] [--mock-permission] [--gnss] [--all] [--api N] out-dir
  recomp  [-jar apktool.jar] out-dir dist/services.jar
  build   dist/services.jar module.zip

Target API levels: 30-34 = Android 11-14 (as the reference tool),
35-37 = Android 15-17 (best-effort, variant-tolerant rules).`)
}

func suiteFlags(fs *flag.FlagSet) (mp, mperm, gnss, all *bool) {
	mp = fs.Bool("mock-provider", false, "apply mock provider suite")
	mperm = fs.Bool("mock-permission", false, "apply mock permission suite")
	gnss = fs.Bool("gnss", false, "apply GNSS suite")
	all = fs.Bool("all", false, "apply all suites")
	return mp, mperm, gnss, all
}

func suiteSet(mp, mperm, gnss, all bool) map[string]bool {
	if all || (!mp && !mperm && !gnss) {
		return map[string]bool{"mock-provider": true, "mock-permission": true, "gnss": true}
	}
	return map[string]bool{"mock-provider": mp, "mock-permission": mperm, "gnss": gnss}
}

func cmdPull(args []string) error {
	fs := flag.NewFlagSet("pull", flag.ExitOnError)
	serial := fs.String("s", "", "device serial")
	out := fs.String("o", "services.jar", "output file")
	fs.Parse(args)
	devs, err := adb.Devices()
	if err != nil {
		return err
	}
	if len(devs) == 0 {
		return fmt.Errorf("no adb device found")
	}
	s := *serial
	if s == "" {
		s = devs[0]
	}
	api, err := adb.APILevel(s)
	if err != nil {
		return err
	}
	fmt.Printf("device %s api %d\n", s, api)
	return adb.Pull(s, "/system/framework/services.jar", *out)
}

// TermuxPrefix is the only supported runtime prefix. This tool does not
// support proot distros or desktop Linux: adb/apktool/java are expected
// from Termux packages.
const TermuxPrefix = "/data/data/com.termux/files/usr"

// apktoolJar resolves the apktool jar, defaulting to the Termux package
// location (/data/data/com.termux/files/usr/share/java/apktool.jar).
func apktoolJar(flagVal string) (string, error) {
	if _, err := os.Stat(flagVal); err == nil {
		return flagVal, nil
	}
	for _, cand := range []string{
		TermuxPrefix + "/share/java/apktool.jar",
		"apktool.jar",
	} {
		if _, err := os.Stat(cand); err == nil {
			return cand, nil
		}
	}
	return "", fmt.Errorf("apktool jar not found: %s (pass -jar)", flagVal)
}
func cmdDisasm(args []string) error {
	fs := flag.NewFlagSet("disasm", flag.ExitOnError)
	jarFlag := fs.String("jar", "apktool.jar", "apktool jar (default: Termux package path)")
	api := fs.Int("api", 0, "target api level, recorded for patch/verify (apktool 3.x auto-detects)")
	fs.Parse(args)
	rest := fs.Args()
	if len(rest) != 2 {
		return fmt.Errorf("usage: smalipatcher disasm [-jar apktool.jar] [-api N] services.jar out-dir")
	}
	apktool, out := rest[0], rest[1]
	if _, err := os.Stat(apktool); err != nil {
		return fmt.Errorf("services.jar not found: %s", apktool)
	}
	jarPath, err := apktoolJar(*jarFlag)
	if err != nil {
		return err
	}
	// apktool 3.x decode flags (the 2.x "-api" decode flag is gone;
	// --api is only recorded here for the patch/verify stage).
	argv := []string{"-jar", jarPath, "d", "--no-debug-info", "-f", "--keep-broken-res", "-o", out, apktool}
	_ = api
	cmd := exec.Command("java", argv...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("apktool decompile failed: %w", err)
	}
	return nil
}

func cmdRecomp(args []string) error {
	fs := flag.NewFlagSet("recomp", flag.ExitOnError)
	jarFlag := fs.String("jar", "apktool.jar", "apktool jar (default: Termux package path)")
	fs.Parse(args)
	rest := fs.Args()
	if len(rest) != 2 {
		return fmt.Errorf("usage: smalipatcher recomp [-jar apktool.jar] out-dir dist/services.jar")
	}
	out, dist := rest[0], rest[1]
	jarPath, err := apktoolJar(*jarFlag)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dist), 0o755); err != nil {
		return err
	}
	argv := []string{"-jar", jarPath, "b", "-f", "-o", dist, out}
	cmd := exec.Command("java", argv...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("apktool rebuild failed: %w", err)
	}
	return nil
}

func report(results []patcher.Result) int {
	changed := 0
	for _, r := range results {
		mark := "SKIP"
		if r.Changed {
			mark = "PATCHED"
			changed++
		}
		fmt.Printf("[%s] %s :: %s -- %s\n", mark, r.Rule.Suite, r.File, r.Detail)
		if r.Rule.Note != "" && r.Changed {
			fmt.Printf("         %s\n", r.Rule.Note)
		}
	}
	return changed
}

func cmdPatch(args []string) error {
	fs := flag.NewFlagSet("patch", flag.ExitOnError)
	mp, mperm, gnss, all := suiteFlags(fs)
	api := fs.Int("api", 0, "target Android API level (0 = unknown, try all variants; 35-37 = Android 15-17 best-effort)")
	fs.Parse(args)
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: smalipatcher patch [--mock-provider] [--mock-permission] [--gnss] [--all] [--api N] out-dir")
	}
	if *api > 0 {
		fmt.Printf("target API: %d\n", *api)
	}
	results, err := patcher.ApplySuites(fs.Arg(0), suiteSet(*mp, *mperm, *gnss, *all), *api)
	if err != nil {
		return err
	}
	n := report(results)
	fmt.Printf("%d file(s) changed\n", n)
	return nil
}

func cmdVerify(args []string) error {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	mp, mperm, gnss, all := suiteFlags(fs)
	api := fs.Int("api", 0, "target Android API level (0 = unknown, try all variants; 35-37 = Android 15-17 best-effort)")
	fs.Parse(args)
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: smalipatcher verify [--mock-provider] [--mock-permission] [--gnss] [--all] [--api N] out-dir")
	}
	if *api > 0 {
		fmt.Printf("target API: %d\n", *api)
	}
	results, err := patcher.Verify(fs.Arg(0), suiteSet(*mp, *mperm, *gnss, *all), *api)
	if err != nil {
		return err
	}
	failed := 0
	skipped := 0
	for _, r := range results {
		mark := "OK  "
		if r.Skipped {
			mark = "SKIP"
			skipped++
		} else if !r.Changed {
			mark = "FAIL"
			failed++
		}
		fmt.Printf("[%s] %s :: %s -- %s\n", mark, r.Rule.Suite, r.File, r.Detail)
	}
	if failed > 0 {
		return fmt.Errorf("%d check(s) failed (%d skipped)", failed, skipped)
	}
	fmt.Printf("all applicable checks passed (%d skipped)\n", skipped)
	return nil
}

func cmdBuild(args []string) error {
	fs := flag.NewFlagSet("build", flag.ExitOnError)
	fs.Parse(args)
	if fs.NArg() != 2 {
		return fmt.Errorf("usage: smalipatcher build patched-services.jar module.zip")
	}
	if err := magisk.Build(fs.Arg(0), fs.Arg(1)); err != nil {
		return err
	}
	fmt.Printf("wrote %s\n", fs.Arg(1))
	return nil
}
