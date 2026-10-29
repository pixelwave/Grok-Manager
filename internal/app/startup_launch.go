package app

import "strings"

// startupCommand is the sign-in command Task Manager lists under Startup apps.
// The value is the app itself. A powershell.exe command is omitted from that list.
func startupCommand(exe string) string {
	return `"` + strings.Trim(exe, `"`) + `"`
}

func startupValueNeedsExe(command string) bool {
	c := strings.ToLower(strings.TrimSpace(command))
	return strings.Contains(c, "powershell") ||
		strings.Contains(c, "start-hidden.ps1") ||
		strings.Contains(c, "grok-manager.cmd")
}
