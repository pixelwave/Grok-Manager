package app

import (
	"context"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
)

const serviceType = "_grok-manager._tcp"

var (
	peerMu     sync.Mutex
	peerCache  []Machine
	peerCached time.Time
)

// BrowsePeers returns managers announced on the local network.
// The result is cached briefly. A failure leaves the page working.
func BrowsePeers(port int) []Machine {
	peerMu.Lock()
	defer peerMu.Unlock()
	if time.Since(peerCached) < 20*time.Second {
		return append([]Machine(nil), peerCache...)
	}
	found := browseOnce(port)
	peerCache = found
	peerCached = time.Now()
	return append([]Machine(nil), found...)
}

func browseOnce(port int) []Machine {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil
	}
	entries := make(chan *zeroconf.ServiceEntry)
	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()
	if err := resolver.Browse(ctx, serviceType, "local.", entries); err != nil {
		return nil
	}
	local := localAddrs()
	var found []Machine
	seen := map[string]struct{}{}
	for {
		select {
		case <-ctx.Done():
			return found
		case entry, ok := <-entries:
			if !ok || entry == nil {
				return found
			}
			if entry.Port == port && owns(entry, local) {
				continue
			}
			host := firstAddr(entry)
			if host == "" {
				continue
			}
			url := "http://" + net.JoinHostPort(host, strconv.Itoa(entry.Port)) + "/"
			if _, ok := seen[url]; ok {
				continue
			}
			seen[url] = struct{}{}
			name := strings.TrimSuffix(entry.Instance, "."+serviceType+".local.")
			name = strings.TrimSuffix(name, ".local.")
			if name == "" {
				name = host
			}
			found = append(found, Machine{Name: name, URL: url})
		}
	}
}

func firstAddr(entry *zeroconf.ServiceEntry) string {
	if len(entry.AddrIPv4) > 0 {
		return entry.AddrIPv4[0].String()
	}
	if len(entry.AddrIPv6) > 0 {
		return entry.AddrIPv6[0].String()
	}
	return ""
}

func owns(entry *zeroconf.ServiceEntry, local map[string]struct{}) bool {
	for _, ip := range entry.AddrIPv4 {
		if _, ok := local[ip.String()]; ok {
			return true
		}
	}
	for _, ip := range entry.AddrIPv6 {
		if _, ok := local[ip.String()]; ok {
			return true
		}
	}
	return false
}

func localAddrs() map[string]struct{} {
	found := map[string]struct{}{"127.0.0.1": {}, "::1": {}}
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return found
	}
	for _, addr := range addrs {
		switch value := addr.(type) {
		case *net.IPNet:
			found[value.IP.String()] = struct{}{}
		case *net.IPAddr:
			found[value.IP.String()] = struct{}{}
		}
	}
	return found
}

// Announce publishes this manager. The caller ignores a failure.
func Announce(name string, port int, version string) (func(), error) {
	if name == "" {
		name, _ = os.Hostname()
	}
	name = strings.Split(name, ".")[0]
	server, err := zeroconf.Register(name, serviceType, "local.", port, []string{"ver=" + version}, advertiseIfaces())
	if err != nil {
		return func() {}, err
	}
	return server.Shutdown, nil
}

func advertiseIfaces() []net.Interface {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}
	var out []net.Interface
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagMulticast == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		name := iface.Name
		if strings.HasPrefix(name, "docker") || strings.HasPrefix(name, "br-") || strings.HasPrefix(name, "veth") || strings.HasPrefix(name, "virbr") {
			continue
		}
		out = append(out, iface)
	}
	return out
}
