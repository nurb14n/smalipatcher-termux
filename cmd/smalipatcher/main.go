// Command smalipatcher is a terminal rewrite of the SmaliPatcherEx
// mock-provider / mock-permission patch flow:
//
//	smalipatcher pull    [-s serial] [-o services.jar]
//	smalipatcher disasm  [-jar apktool.jar] [-api N] services.jar out-dir
//	smalipatcher patch   [--mock-provider] [--mock-permission] [--gnss] [--all] out-dir
//	smalipatcher verify  [--mock-provider] [--mock-permission] [--gnss] [--all] out-dir
//	smalipatcher build   patched-services.jar module.zip
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"

	"github.com/nurb14n/smalipatcher-app/internal/adb"
	"github.com/nurb14n/smalipatcher-app/internal/magisk"
	"github.com/nurb14n/smalipatcher-app/internal/patcher"
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
	fmt.Println(`usage: smalipatcher <pull|disasm|patch|verify|build> [flags]

  pull    [-s serial] [-o services.jar]   pull services.jar from device
  disasm  [-jar apktool.jar] [-api N] services.jar out-dir
  patch   [--mock-provider] [--mock-permission] [--gnss] [--all] [--api N] out-dir
  verify  [--mock-provider] [--mock-permission] [--gnss] [--all] [--api N] out-dir
  build   patched-services.jar module.zip

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

func cmdDisasm(args []string) error {
	fs := flag.NewFlagSet("disasm", flag.ExitOnError)
	jar := fs.String("jar", "apktool.jar", "apktool jar")
	api := fs.Int("api", 0, "android api level (0 = auto-detect, needs adb device)")
	fs.Parse(args)
	rest := fs.Args()
	if len(rest) != 2 {
		return fmt.Errorf("usage: smalipatcher disasm [-jar apktool.jar] [-api N] services.jar out-dir")
	}
	apktool, out := rest[0], rest[1]
	if _, err := os.Stat(apktool); err != nil {
		return fmt.Errorf("services.jar not found: %s", apktool)
	}
	argv := []string{"-jar", *jar, "d", "-b", "-f", "-k"}
	if *api > 0 {
		argv = append(argv, "-api", fmt.Sprint(*api))
	}
	argv = append(argv, "-o", out, apktool)
	cmd := exec.Command("java", argv...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("apktool decompile failed: %w", err)
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
