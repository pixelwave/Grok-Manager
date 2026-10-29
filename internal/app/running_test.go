package app

import (
	"strings"
	"testing"
)

func TestStatusText(t *testing.T) {
	setup(t)
	t.Setenv("GROK_MANAGER_PORT", "18090")

	text := StatusText("v26.10.29")
	art := " ██████╗ ██████╗  ██████╗ ██╗  ██╗\n" +
		"██╔════╝ ██╔══██╗██╔═══██╗██║ ██╔╝\n" +
		"██║  ███╗██████╔╝██║   ██║█████╔╝ \n" +
		"██║   ██║██╔══██╗██║   ██║██╔═██╗ \n" +
		"╚██████╔╝██║  ██║╚██████╔╝██║  ██╗\n" +
		" ╚═════╝ ╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═╝\n\n\n"
	if !strings.HasPrefix(text, art) {
		t.Fatal("banner")
	}
	if !strings.Contains(text, " Grok Session Manager\n v26.10.29\n") {
		t.Fatal("version block")
	}
	host := " Host      : " + MachineName() + "\n"
	url := " URL       : " + lanIPv4() + ":18090\n"
	block := host + url + "\n * Documentation:  https://github.com/pixelwave/Grok-Manager\n"
	if !strings.HasSuffix(text, block) {
		t.Fatal("fields")
	}
	if strings.Count(text, "v26.10.29") != 1 {
		t.Fatal("version once")
	}

	t.Setenv("GROK_MANAGER_PORT", "6969")
	again := StatusText("26.10.29.2")
	if !strings.Contains(again, "\n v26.10.29.2\n") || !strings.Contains(again, "\n URL       : "+lanIPv4()+":6969\n") {
		t.Fatal("custom port")
	}
	if strings.Contains(again, " IP        :") || strings.Contains(again, " Port      :") {
		t.Fatal("old address lines")
	}
	if !strings.Contains(StatusText("  "), "\n v0\n") {
		t.Fatal("empty version")
	}
}
