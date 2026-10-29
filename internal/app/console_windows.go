//go:build windows

package app

import (
	"os"

	"golang.org/x/sys/windows"
)

// EnableCLIConsole attaches the parent terminal so status and install can print.
// The Windows build has no console of its own, so sign-in startup stays windowless.
func EnableCLIConsole() {
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	r, _, _ := kernel32.NewProc("AttachConsole").Call(^uintptr(0))
	if r == 0 {
		return
	}
	reopenStd(kernel32, ^uintptr(9), &os.Stdin, "stdin")    // STD_INPUT_HANDLE (-10)
	reopenStd(kernel32, ^uintptr(10), &os.Stdout, "stdout") // STD_OUTPUT_HANDLE (-11)
	reopenStd(kernel32, ^uintptr(11), &os.Stderr, "stderr") // STD_ERROR_HANDLE (-12)
}

func reopenStd(kernel32 *windows.LazyDLL, which uintptr, dest **os.File, name string) {
	h, _, _ := kernel32.NewProc("GetStdHandle").Call(which)
	if h == 0 || h == ^uintptr(0) {
		return
	}
	*dest = os.NewFile(h, name)
}

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
