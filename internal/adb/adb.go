// Package adb wraps the small set of adb invocations the patch flow needs:
// device detection, API-level readout, and pulling services.jar.
package adb

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var reAPI = regexp.MustCompile(`API-(\d{2})`)

// Devices returns the list of attached device serials (adb devices -l).
func Devices() ([]string, error) {
	out, err := exec.Command("adb", "devices").Output()
	if err != nil {
		return nil, fmt.Errorf("adb devices: %w (is adb installed and on PATH?)", err)
	}
	var devs []string
	for _, line := range strings.Split(string(out), "\n")[1:] {
		f := strings.Fields(line)
		if len(f) >= 2 && f[1] == "device" {
			devs = append(devs, f[0])
		}
	}
	return devs, nil
}

// APILevel reads ro.build.version.sdk from the device.
func APILevel(serial string) (int, error) {
	args := []string{"shell", "getprop", "ro.build.version.sdk"}
	if serial != "" {
		args = append([]string{"-s", serial}, args...)
	}
	out, err := exec.Command("adb", args...).Output()
	if err != nil {
		return 0, fmt.Errorf("getprop: %w", err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, fmt.Errorf("parse api level %q: %w", strings.TrimSpace(string(out)), err)
	}
	return n, nil
}

// ParseAPI extracts the API level from a string like "API-34".
// (SmaliPatcherEx matches API-(\d{2}) on device info output.)
func ParseAPI(s string) (int, error) {
	m := reAPI.FindStringSubmatch(s)
	if m == nil {
		return 0, fmt.Errorf("no API level in %q", s)
	}
	return strconv.Atoi(m[1])
}

// Pull copies a remote path to a local destination.
func Pull(serial, remote, local string) error {
	args := []string{"pull", remote, local}
	if serial != "" {
		args = append([]string{"-s", serial}, args...)
	}
	if out, err := exec.Command("adb", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("adb pull: %w\n%s", err, out)
	}
	return nil
}
