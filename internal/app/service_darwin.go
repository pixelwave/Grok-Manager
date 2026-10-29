//go:build darwin

package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

func plistPath() string {
	if os.Geteuid() == 0 {
		return "/Library/LaunchDaemons/de.pixelwave.grok-manager.plist"
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", "de.pixelwave.grok-manager.plist")
}

func plistText(exe string, port int) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>Label</key><string>de.pixelwave.grok-manager</string>
<key>ProgramArguments</key><array><string>%s</string></array>
<key>EnvironmentVariables</key><dict><key>GROK_MANAGER_PORT</key><string>%d</string></dict>
<key>RunAtLoad</key><true/>
<key>KeepAlive</key><true/>
</dict></plist>
`, exe, port)
}

func domain() string {
	if os.Geteuid() == 0 {
		return "system"
	}
	return "gui/" + strconv.Itoa(os.Getuid())
}

func InstallService(port int) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	path := plistPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(plistText(exe, port)), 0o644); err != nil {
		return err
	}
	_ = exec.Command("launchctl", "bootout", domain(), path).Run()
	return exec.Command("launchctl", "bootstrap", domain(), path).Run()
}

func StartService() error { return InstallService(LoadPort()) }
func StopService() error {
	return exec.Command("launchctl", "bootout", domain(), plistPath()).Run()
}

func RestartAfterUpdate() {
	time.Sleep(400 * time.Millisecond)
	_ = exec.Command("launchctl", "kickstart", "-k", domain()+"/de.pixelwave.grok-manager").Run()
	exe, err := os.Executable()
	if err != nil {
		return
	}
	_ = syscall.Exec(exe, os.Args, os.Environ())
}

func Uninstall(purge bool) error {
	_ = StopService()
	_ = os.Remove(plistPath())
	if purge {
		_ = os.RemoveAll(ConfigDir())
	}
	if exe, err := os.Executable(); err == nil {
		_ = os.Remove(exe)
	}
	return nil
}
