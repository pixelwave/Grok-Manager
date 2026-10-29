//go:build linux

package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

func unitPath() string {
	if os.Geteuid() == 0 {
		return "/etc/systemd/system/grok-manager.service"
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "systemd", "user", "grok-manager.service")
}

func unitText(exe string, port int) string {
	wanted := "default.target"
	if os.Geteuid() == 0 {
		wanted = "multi-user.target"
	}
	return fmt.Sprintf(`[Unit]
Description=Grok Manager
After=network.target

[Service]
ExecStart=%s
Environment=GROK_MANAGER_PORT=%d
Restart=on-failure
RestartSec=2

[Install]
WantedBy=%s
`, exe, port, wanted)
}

func systemctl(args ...string) error {
	if os.Geteuid() != 0 {
		args = append([]string{"--user"}, args...)
	}
	return exec.Command("systemctl", args...).Run()
}

func InstallService(port int) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	path := unitPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(unitText(exe, port)), 0o644); err != nil {
		return err
	}
	_ = systemctl("daemon-reload")
	if err := systemctl("enable", "grok-manager.service"); err != nil {
		return err
	}
	// enable --now leaves an already running service on its old process.
	if serviceActive() {
		return systemctl("restart", "grok-manager.service")
	}
	return systemctl("start", "grok-manager.service")
}

func serviceActive() bool {
	args := []string{"is-active", "--quiet", "grok-manager.service"}
	if os.Geteuid() != 0 {
		args = append([]string{"--user"}, args...)
	}
	return exec.Command("systemctl", args...).Run() == nil
}

func StartService() error {
	return systemctl("enable", "--now", "grok-manager.service")
}

func StopService() error {
	return systemctl("disable", "--now", "grok-manager.service")
}

func RestartAfterUpdate() {
	time.Sleep(400 * time.Millisecond)
	if err := systemctl("restart", "grok-manager.service"); err == nil {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	_ = syscall.Exec(exe, os.Args, os.Environ())
}

func removeProgram() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	_ = os.Remove(exe)
	_ = os.Remove(exe + ".old")
}

func Uninstall(purge bool) error {
	_ = StopService()
	_ = os.Remove(unitPath())
	_ = systemctl("daemon-reload")
	if purge {
		_ = os.RemoveAll(ConfigDir())
	}
	removeProgram()
	return nil
}
