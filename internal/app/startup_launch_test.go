package app

import "testing"

func TestStartupCommand(t *testing.T) {
	exe := `C:\Users\info\AppData\Local\grok-manager\grok-manager.exe`
	got := startupCommand(exe)
	want := `"C:\Users\info\AppData\Local\grok-manager\grok-manager.exe"`
	if got != want {
		t.Fatalf("command %q", got)
	}
	if startupCommand(want) != want {
		t.Fatalf("quoted twice: %q", startupCommand(want))
	}
	if !startupValueNeedsExe(`powershell.exe -NoProfile -File "C:\App\start-hidden.ps1"`) {
		t.Fatal("powershell launcher should be replaced")
	}
	if !startupValueNeedsExe(`C:\App\grok-manager.cmd`) {
		t.Fatal("cmd launcher should be replaced")
	}
	if startupValueNeedsExe(want) {
		t.Fatal("direct exe should stay")
	}
}
