//go:build windows

package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func startupFile() string {
	app := os.Getenv("APPDATA")
	return filepath.Join(app, "Microsoft", "Windows", "Start Menu", "Programs", "Startup", "grok-manager.cmd")
}

func InstallService(port int) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(startupFile()), 0o755); err != nil {
		return err
	}
	body := fmt.Sprintf("@echo off\r\nset GROK_MANAGER_PORT=%d\r\nstart \"\" \"%s\"\r\n", port, exe)
	if err := os.WriteFile(startupFile(), []byte(body), 0o644); err != nil {
		return err
	}
	return StartService()
}

func StartService() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe)
	cmd.Env = append(os.Environ(), fmt.Sprintf("GROK_MANAGER_PORT=%d", LoadPort()))
	return cmd.Start()
}

func StopService() error {
	_ = exec.Command("taskkill", "/IM", "grok-manager.exe", "/F").Run()
	return nil
}

func RestartAfterUpdate() {
	time.Sleep(400 * time.Millisecond)
	exe, err := os.Executable()
	if err != nil {
		return
	}
	cmd := exec.Command(exe)
	cmd.Env = os.Environ()
	_ = cmd.Start()
	os.Exit(0)
}

func Uninstall(purge bool) error {
	_ = StopService()
	_ = os.Remove(startupFile())
	if purge {
		_ = os.RemoveAll(ConfigDir())
	}
	if exe, err := os.Executable(); err == nil {
		_ = os.Remove(exe)
	}
	return nil
}
