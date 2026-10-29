package app

import (
	"fmt"
	"net"
	"time"
)

func ServiceRunning() bool {
	port := LoadPort()
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 300*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func StatusText(version string) string {
	port := LoadPort()
	state := "stopped"
	if ServiceRunning() {
		state = "running"
	}
	return fmt.Sprintf("Grok Manager %s\nstatus: %s\nport: %d\nurl: http://127.0.0.1:%d\nconfig: %s\n", version, state, port, port, ConfigDir())
}
