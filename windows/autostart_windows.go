//go:build windows

package main

import (
	"os"
	"path/filepath"

	"golang.org/x/sys/windows/registry"
)

const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`
const appName = "SnowdenSystem"

// setAutostartRegistry adds or removes the app from HKCU Run key.
func setAutostartRegistry(enabled bool) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()

	if enabled {
		exePath, err := os.Executable()
		if err != nil {
			return err
		}
		// Resolve to absolute path (Windows needs the full path + flag).
		val := `"` + filepath.Join(filepath.Dir(exePath), filepath.Base(exePath)) + `"`
		return k.SetStringValue(appName, val)
	}
	// Remove the value if it exists.
	return k.DeleteValue(appName)
}

// isAutostartEnabled reports whether the app is in the Run key.
func isAutostartEnabled() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	_, _, err = k.GetStringValue(appName)
	return err == nil
}
