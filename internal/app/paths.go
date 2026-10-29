package app

import (
	"os"
	"path/filepath"
	"runtime"
)

// ConfigDir holds locks, pins, machines, and the chosen port.
// It is outside the install directory so an update can replace the binary.
func ConfigDir() string {
	if runtime.GOOS == "windows" {
		if app := os.Getenv("APPDATA"); app != "" {
			return filepath.Join(app, "grok-manager")
		}
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "grok-manager")
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return filepath.Join(".", ".config", "grok-manager")
	}
	return filepath.Join(home, ".config", "grok-manager")
}

func configFile(env, name string) string {
	if value := os.Getenv(env); value != "" {
		return value
	}
	return filepath.Join(ConfigDir(), name)
}

func locksPath() string { return configFile("GROK_LOCKS_PATH", "locks.json") }
func pinsPath() string  { return configFile("GROK_PINS_PATH", "pins.json") }
func machinesPath() string {
	return configFile("GROK_MACHINES_PATH", "machines.json")
}
func settingsPath() string { return configFile("GROK_SETTINGS_PATH", "settings.json") }
