//go:build windows

package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func startupFile() string {
	app := os.Getenv("APPDATA")
	return filepath.Join(app, "Microsoft", "Windows", "Start Menu", "Programs", "Startup", "grok-manager.cmd")
}

func startupLink() string {
	return filepath.Join(filepath.Dir(startupFile()), "Grok Manager.lnk")
}

func InstallService(port int) error {
	if port <= 0 || port > 65535 {
		return fmt.Errorf("invalid port")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	removeOldLaunchers(exe)
	removeLogonTask()
	if err := setStartupApp(exe, true); err != nil {
		return err
	}
	if err := ensureOnUserPath(filepath.Dir(exe)); err != nil {
		return err
	}
	return startHidden(exe)
}

func setStartupApp(exe string, enabled bool) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	if err := key.SetStringValue("Grok Manager", startupCommand(exe)); err != nil {
		return err
	}
	return setStartupApproved(enabled)
}

func setStartupApproved(enabled bool) error {
	approved, _, err := registry.CreateKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run`, registry.SET_VALUE)
	if err != nil {
		return nil
	}
	defer approved.Close()
	flag := byte(2)
	if !enabled {
		flag = 3
	}
	return approved.SetBinaryValue("Grok Manager", []byte{flag, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})
}

// MigrateStartupApp points an older PowerShell sign-in command at this exe.
// Task Manager lists that Run entry as Grok Manager. The enabled flag is kept.
func MigrateStartupApp() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.QUERY_VALUE|registry.SET_VALUE)
	if err != nil {
		return
	}
	defer key.Close()
	current, _, err := key.GetStringValue("Grok Manager")
	if err != nil || !startupValueNeedsExe(current) {
		return
	}
	_ = key.SetStringValue("Grok Manager", startupCommand(exe))
	removeOldLaunchers(exe)
}

func removeStartupApp() {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if err == nil {
		_ = key.DeleteValue("Grok Manager")
		key.Close()
	}
	approved, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\Run`, registry.SET_VALUE)
	if err == nil {
		_ = approved.DeleteValue("Grok Manager")
		approved.Close()
	}
}

func removeOldLaunchers(exe string) {
	_ = os.Remove(startupFile())
	_ = os.Remove(startupLink())
	_ = os.Remove(filepath.Join(filepath.Dir(exe), "start-hidden.ps1"))
	approved, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Explorer\StartupApproved\StartupFolder`, registry.SET_VALUE)
	if err == nil {
		_ = approved.DeleteValue("Grok Manager.lnk")
		approved.Close()
	}
}

func startHidden(exe string) error {
	cmd := exec.Command(exe)
	cmd.Env = append(os.Environ(), fmt.Sprintf("GROK_MANAGER_PORT=%d", LoadPort()))
	cmd.SysProcAttr = detachedProcess()
	return cmd.Start()
}

func removeLogonTask() {
	_ = powershell("Unregister-ScheduledTask -TaskName 'Grok Manager' -Confirm:$false -ErrorAction SilentlyContinue")
}

func powershell(script string) error {
	cmd := exec.Command("powershell", "-NoProfile", "-Command", script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
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
	if err := setStartupApp(exe, true); err != nil {
		return err
	}
	return startHidden(exe)
}

func detachedProcess() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: 0x00000008} // DETACHED_PROCESS
}

func StopService() error {
	exe, err := os.Executable()
	if err == nil {
		_ = setStartupApp(exe, false)
	}
	_ = exec.Command("taskkill", "/IM", "grok-manager.exe", "/F").Run()
	return nil
}

func RestartAfterUpdate() {
	time.Sleep(400 * time.Millisecond)
	if exe, err := os.Executable(); err == nil {
		_ = startHidden(exe)
	}
	os.Exit(0)
}

func Uninstall(purge bool) error {
	_ = StopService()
	removeLogonTask()
	removeStartupApp()
	if exe, err := os.Executable(); err == nil {
		removeOldLaunchers(exe)
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
