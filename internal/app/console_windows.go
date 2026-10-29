//go:build windows

package app

import "golang.org/x/sys/windows"

// HideServerConsole removes the console window for the background page.
// Closing that window would stop the process.
func HideServerConsole() {
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	user32 := windows.NewLazySystemDLL("user32.dll")
	hwnd, _, _ := kernel32.NewProc("GetConsoleWindow").Call()
	if hwnd == 0 {
		return
	}
	const swHide = 0
	_, _, _ = user32.NewProc("ShowWindow").Call(hwnd, swHide)
}
