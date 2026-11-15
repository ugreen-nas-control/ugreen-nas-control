package mqtt

import (
	"net"
	"os"
	"path/filepath"
)

// addrsOf lists the addresses of an interface (replaced in tests).
var addrsOf = func(name string) []net.Addr {
	ifc, err := net.InterfaceByName(name)
	if err != nil {
		return nil
	}
	a, _ := ifc.Addrs()
	return a
}

func firstIPv4(addrs []net.Addr) string {
	for _, a := range addrs {
		ipn, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		if ip := ipn.IP.To4(); ip != nil && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() {
			return ip.String()
		}
	}
	return ""
}

// ifaceIPv4 is the IPv4 address of a port. Unraid usually puts it further up, on a bridge or a bond (eth0 belongs to
// bond0, bond0 to br0), so the port's master, and that one's master, is tried until an address turns up.
func ifaceIPv4(name string) string {
	for i := 0; i < 4 && name != ""; i++ {
		if ip := firstIPv4(addrsOf(name)); ip != "" {
			return ip
		}
		l, err := os.Readlink(filepath.Join(sysDir, "class/net", name, "master"))
		if err != nil {
			return ""
		}
		name = filepath.Base(l)
	}
	return ""
}
