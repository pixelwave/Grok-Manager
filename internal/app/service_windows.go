//go:build windows

package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
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
	if err := ensureOnUserPath(filepath.Dir(exe)); err != nil {
		return err
	}
	return StartService()
}

func ensureOnUserPath(dir string) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	current, kind, err := key.GetStringValue("Path")
	if err == registry.ErrNotExist {
		current, kind, err = "", registry.EXPAND_SZ, nil
	}
	if err != nil {
		return err
	}
	next := pathListAdd(current, dir)
	if next == current {
		return nil
	}
	if kind == registry.SZ {
		err = key.SetStringValue("Path", next)
	} else {
		err = key.SetExpandStringValue("Path", next)
	}
	if err != nil {
		return err
	}
	notifyEnvironment()
	return nil
}

func removeFromUserPath(dir string) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Environment`, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	current, kind, err := key.GetStringValue("Path")
	if err == registry.ErrNotExist {
		return nil
	}
	if err != nil {
		return err
	}
	next := pathListRemove(current, dir)
	if next == current {
		return nil
	}
	if kind == registry.SZ {
		err = key.SetStringValue("Path", next)
	} else {
		err = key.SetExpandStringValue("Path", next)
	}
	if err != nil {
		return err
	}
	notifyEnvironment()
	return nil
}

func notifyEnvironment() {
	env, err := windows.UTF16PtrFromString("Environment")
	if err != nil {
		return
	}
	const hwndBroadcast = 0xffff
	const wmSettingChange = 0x001A
	const smtoAbortIfHung = 0x0002
	proc := windows.NewLazySystemDLL("user32.dll").NewProc("SendMessageTimeoutW")
	_, _, _ = proc.Call(hwndBroadcast, wmSettingChange, 0, uintptr(unsafe.Pointer(env)), smtoAbortIfHung, 5000, 0)
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
	if exe, err := os.Executable(); err == nil {
		_ = removeFromUserPath(filepath.Dir(exe))
	}
	if purge {
		_ = os.RemoveAll(ConfigDir())
	}
	if exe, err := os.Executable(); err == nil {
		_ = os.Remove(exe)
	}
	return nil
}
