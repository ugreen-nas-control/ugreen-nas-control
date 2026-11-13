package mqtt

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestIfaceIPv4(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "class/net/eth0"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../br0", filepath.Join(dir, "class/net/eth0/master")); err != nil {
		t.Fatal(err)
	}
	oldSys, oldAddrs := sysDir, addrsOf
	defer func() { sysDir, addrsOf = oldSys, oldAddrs }()
	sysDir = dir
	ip := func(s string) net.Addr { _, n, _ := net.ParseCIDR(s); n.IP = net.ParseIP(s[:len(s)-3]); return n }
	table := map[string][]net.Addr{
		"br0":  {ip("fe80::1/64"), ip("192.168.40.101/24")},
		"eth1": {ip("169.254.3.4/16"), ip("127.0.0.1/8")},
		"eth2": {ip("10.0.0.5/24")},
	}
	addrsOf = func(n string) []net.Addr { return table[n] }
	if got := ifaceIPv4("eth0"); got != "192.168.40.101" {
		t.Fatalf("a port without an address uses its bridge: %q", got)
	}
	if got := ifaceIPv4("eth2"); got != "10.0.0.5" {
		t.Fatalf("own address: %q", got)
	}
	if got := ifaceIPv4("eth1"); got != "" {
		t.Fatalf("link-local and loopback are not shown: %q", got)
	}
	// eth3 belongs to bond0, bond0 to br0: the address sits two steps up
	for _, n := range []string{"eth3", "bond0"} {
		if err := os.MkdirAll(filepath.Join(dir, "class/net", n), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("../bond0", filepath.Join(dir, "class/net/eth3/master")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../br0", filepath.Join(dir, "class/net/bond0/master")); err != nil {
		t.Fatal(err)
	}
	if got := ifaceIPv4("eth3"); got != "192.168.40.101" {
		t.Fatalf("a port in a bond in a bridge uses the bridge address: %q", got)
	}
	// masters that point at each other must not loop forever
	for _, n := range []string{"a", "b"} {
		if err := os.MkdirAll(filepath.Join(dir, "class/net", n), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("../b", filepath.Join(dir, "class/net/a/master")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../a", filepath.Join(dir, "class/net/b/master")); err != nil {
		t.Fatal(err)
	}
	if got := ifaceIPv4("a"); got != "" {
		t.Fatalf("a loop of masters has no address: %q", got)
	}
	if got := ifaceIPv4("nope"); got != "" {
		t.Fatalf("unknown port: %q", got)
	}
}
