package app

import (
	"fmt"
	"net"
	"strings"
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
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(version, "v")
	if version == "" {
		version = "0"
	}
	return fmt.Sprintf(` ██████╗ ██████╗  ██████╗ ██╗  ██╗
██╔════╝ ██╔══██╗██╔═══██╗██║ ██╔╝
██║  ███╗██████╔╝██║   ██║█████╔╝ 
██║   ██║██╔══██╗██║   ██║██╔═██╗ 
╚██████╔╝██║  ██║╚██████╔╝██║  ██╗
 ╚═════╝ ╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═╝


 Grok Session Manager
 v%s

 Host      : %s
 IP        : %s
 Port      : %d

 * Documentation:  https://github.com/pixelwave/Grok-Manager
`, version, MachineName(), lanIPv4(), LoadPort())
}

// lanIPv4 is the first private IPv4 on an up, non-virtual interface.
// A public address is used when no private one exists.
func lanIPv4() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "-"
	}
	fallback := ""
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		name := iface.Name
		if strings.HasPrefix(name, "docker") || strings.HasPrefix(name, "br-") || strings.HasPrefix(name, "veth") || strings.HasPrefix(name, "virbr") {
			continue
		}
		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, addr := range addrs {
			ip := addrIP(addr)
			v4 := ip.To4()
			if v4 == nil || v4.IsLoopback() || v4.IsLinkLocalUnicast() {
				continue
			}
			if v4.IsPrivate() {
				return v4.String()
			}
			if fallback == "" && v4.IsGlobalUnicast() {
				fallback = v4.String()
			}
		}
	}
	if fallback == "" {
		return "-"
	}
	return fallback
}

func addrIP(addr net.Addr) net.IP {
	switch value := addr.(type) {
	case *net.IPNet:
		return value.IP
	case *net.IPAddr:
		return value.IP
	default:
		return nil
	}
}
