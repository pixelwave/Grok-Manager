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

func InstallService(port int) error {
	if port <= 0 || port > 65535 {
		return fmt.Errorf("invalid port")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	_ = os.Remove(startupFile())
	launcher, err := writeHiddenLauncher(exe)
	if err != nil {
		return err
	}
	if err := registerLogonTask(launcher); err != nil {
		return err
	}
	if err := ensureOnUserPath(filepath.Dir(exe)); err != nil {
		return err
	}
	return startLogonTask()
}

func writeHiddenLauncher(exe string) (string, error) {
	path := filepath.Join(filepath.Dir(exe), "start-hidden.ps1")
	body := fmt.Sprintf("$psi = New-Object System.Diagnostics.ProcessStartInfo\r\n$psi.FileName = '%s'\r\n$psi.UseShellExecute = $false\r\n$psi.CreateNoWindow = $true\r\n[void][System.Diagnostics.Process]::Start($psi)\r\n", strings.ReplaceAll(exe, "'", "''"))
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func registerLogonTask(launcher string) error {
	script := fmt.Sprintf(`
$action = New-ScheduledTaskAction -Execute 'powershell.exe' -Argument '-NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File \"%s\"'
$trigger = New-ScheduledTaskTrigger -AtLogOn
$settings = New-ScheduledTaskSettingsSet -AllowStartIfOnBatteries -DontStopIfGoingOnBatteries -RestartCount 999 -RestartInterval (New-TimeSpan -Minutes 1) -ExecutionTimeLimit ([TimeSpan]::Zero) -MultipleInstances IgnoreNew
Register-ScheduledTask -TaskName 'Grok Manager' -Action $action -Trigger $trigger -Settings $settings -Force | Out-Null
`, launcher)
	return powershell(script)
}

func startLogonTask() error {
	return powershell("Start-ScheduledTask -TaskName 'Grok Manager'")
}

func stopLogonTask() {
	_ = powershell("Stop-ScheduledTask -TaskName 'Grok Manager' -ErrorAction SilentlyContinue; Disable-ScheduledTask -TaskName 'Grok Manager' -ErrorAction SilentlyContinue | Out-Null")
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
	if err := powershell("Enable-ScheduledTask -TaskName 'Grok Manager' | Out-Null"); err == nil {
		return startLogonTask()
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe)
	cmd.Env = append(os.Environ(), fmt.Sprintf("GROK_MANAGER_PORT=%d", LoadPort()))
	cmd.SysProcAttr = detachedProcess()
	return cmd.Start()
}

func detachedProcess() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: 0x00000008} // DETACHED_PROCESS
}

func StopService() error {
	stopLogonTask()
	_ = exec.Command("taskkill", "/IM", "grok-manager.exe", "/F").Run()
	return nil
}

func RestartAfterUpdate() {
	time.Sleep(400 * time.Millisecond)
	if startLogonTask() != nil {
		if exe, err := os.Executable(); err == nil {
			cmd := exec.Command(exe)
			cmd.Env = os.Environ()
			cmd.SysProcAttr = detachedProcess()
			_ = cmd.Start()
		}
	}
	os.Exit(0)
}

func Uninstall(purge bool) error {
	_ = StopService()
	removeLogonTask()
	_ = os.Remove(startupFile())
	if exe, err := os.Executable(); err == nil {
		_ = removeFromUserPath(filepath.Dir(exe))
		_ = os.Remove(filepath.Join(filepath.Dir(exe), "start-hidden.ps1"))
	}
	if purge {
		_ = os.RemoveAll(ConfigDir())
	}
	if exe, err := os.Executable(); err == nil {
		_ = os.Remove(exe)
	}
	return nil
}
