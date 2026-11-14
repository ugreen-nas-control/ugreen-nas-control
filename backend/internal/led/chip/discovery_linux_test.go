//go:build linux

package chip

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"syscall"
	"testing"

	"github.com/ugreen-nas-control/ugreen-nas-control/internal/hw"
)

func addNativeAdapter(t *testing.T, n, name string) string {
	t.Helper()
	dir := filepath.Join(i2cSysBus, "i2c-"+n)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "name"), []byte(name), 0o644); err != nil {
		t.Fatal(err)
	}
	dev := filepath.Join(i2cDevDir, "i2c-"+n)
	if err := os.WriteFile(dev, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	return dev
}

func TestDiscoveryTriesNativeAdaptersBeforeLPSS(t *testing.T) {
	box := newAutoBox(t, "", "nak", sysOpts{})
	one := addNativeAdapter(t, "1", dwAdapter)
	ten := addNativeAdapter(t, "10", dwAdapter)
	two := addNativeAdapter(t, "2", dwAdapter)
	addNativeAdapter(t, "3", "i915 gmbus dpa")
	addNativeAdapter(t, "4", "AMDGPU DM i2c hw bus 0")
	var asked []string
	nativeProbe = func(path string, _ bool) error {
		asked = append(asked, path)
		if path == ten {
			return nil
		}
		return syscall.ENXIO
	}
	got, err := findBus()
	want := []string{one, two, ten}
	if err != nil || got != ten || !reflect.DeepEqual(asked, want) {
		t.Fatalf("findBus = %q, %v; asked %v, want %v", got, err, asked, want)
	}
	if box.asked != 1 || !box.untouched() {
		t.Fatal("native success must only perform the fast I801 check, never map LPSS")
	}
}

func TestDiscoveryPrefersHT32Adapter(t *testing.T) {
	newAutoBox(t, "", "ack", sysOpts{})
	addNativeAdapter(t, "2", dwAdapter)
	adapter := filepath.Join(i2cSysBus, "i2c-2")
	mcu := filepath.Join(adapter, "i2c-HT32F522:00")
	if err := os.Mkdir(mcu, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(mcu, filepath.Join(i2cSysBus, "i2c-HT32F522:00")); err != nil {
		t.Fatal(err)
	}
	var asked []string
	nativeProbe = func(path string, _ bool) error { asked = append(asked, path); return nil }
	got, err := findBus()
	want := filepath.Join(i2cDevDir, "i2c-2")
	if err != nil || got != want || !reflect.DeepEqual(asked, []string{want}) {
		t.Fatalf("findBus = %q, %v; asked %v", got, err, asked)
	}
}

func TestDiscoveryContinuesAfterMissingNodeOrBusyBus(t *testing.T) {
	newAutoBox(t, "", "", sysOpts{noMCU: true})
	zero := addNativeAdapter(t, "0", "SMBus I801 adapter")
	one := addNativeAdapter(t, "1", dwAdapter)
	two := addNativeAdapter(t, "2", dwAdapter)
	if err := os.Remove(zero); err != nil {
		t.Fatal(err)
	}
	nativeProbe = func(path string, _ bool) error {
		if path == one {
			return syscall.EBUSY
		}
		return nil
	}
	if got, err := findBus(); err != nil || got != two {
		t.Fatalf("findBus = %q, %v", got, err)
	}
}

func TestDiscoveryDoesNotSelectAnUnverifiedAdapter(t *testing.T) {
	newAutoBox(t, "", "", sysOpts{noMCU: true})
	addNativeAdapter(t, "1", dwAdapter)
	nativeProbe = func(string, bool) error { return errBadStatus }
	if got, err := findBus(); got != "" || !errors.Is(err, errBadStatus) {
		t.Fatalf("findBus = %q, %v", got, err)
	}
	nativeProbe = func(string, bool) error { return ErrRejected }
	if _, err := findBus(); !errors.Is(err, ErrRejected) {
		t.Fatalf("lost rejected-controller error: %v", err)
	}
}

func TestDiscoveryMissingNodeHasActionableError(t *testing.T) {
	newAutoBox(t, "", "ack", sysOpts{noMCU: true})
	if err := os.Remove(filepath.Join(i2cDevDir, "i2c-0")); err != nil {
		t.Fatal(err)
	}
	if _, err := findBus(); err == nil || !strings.Contains(err.Error(), "modprobe i2c-dev") {
		t.Fatalf("findBus error = %v", err)
	}
}

func TestPowerOnlyMCUDoesNotProbeAt3a(t *testing.T) {
	newAutoBox(t, "", "ack", sysOpts{noMCU: true})
	nativeProbe = func(string, bool) error { t.Fatal("probed 0x3a for the power-only MCU"); return syscall.ENXIO }
	if path, err := findBusAt(dxp480tAddr, false); err != nil || path != filepath.Join(i2cDevDir, "i2c-0") {
		t.Fatalf("findBusAt = %q, %v", path, err)
	}
}

func TestNativeTransportDoesNotChooseTheWriteFrame(t *testing.T) {
	old := getLayout()
	t.Cleanup(func() { SetLayout(old) })
	for _, tc := range []struct{ product, frame string }{
		{"DX4600", "legacy"}, {"DX4600 Pro", "legacy"}, {"DXP4800 Plus", "legacy"},
		{"iDX6011", "idx"}, {"iDX6011 Pro", "idx"}, {"DXP4800 GT", "gt"},
		{"DXP4800 GT rev2", "gt"}, {"DXP4800GT", "gt"}, {"DXP4800 Pro", "legacy"},
	} {
		UseProduct(tc.product)
		if got := framesFor(false)[0].name; got != tc.frame {
			t.Errorf("%s: %s, want %s", tc.product, got, tc.frame)
		}
	}
}

type fakeNativeIdentity struct {
	statusErr error
	ids       []uint16
	idErr     error
	reads     int
}

func (f *fakeNativeIdentity) status(string) (ledStatus, error) { return ledStatus{}, f.statusErr }
func (f *fakeNativeIdentity) readWord(reg byte) (uint16, error) {
	f.reads++
	if reg != 0x5a {
		return 0, syscall.EINVAL
	}
	if len(f.ids) == 0 {
		return 0, f.idErr
	}
	id := f.ids[0]
	f.ids = f.ids[1:]
	return id, f.idErr
}

func TestNativeMCUIdentityIsReadOnlyAndRetries(t *testing.T) {
	for _, tc := range []struct {
		name      string
		reader    fakeNativeIdentity
		wantErr   error
		wantReads int
	}{
		{"valid status", fakeNativeIdentity{}, nil, 0},
		{"signature after bad status", fakeNativeIdentity{statusErr: errBadStatus, ids: []uint16{0xc5b2}}, nil, 1},
		{"signature after retry", fakeNativeIdentity{statusErr: errBadStatus, ids: []uint16{0, 0, 0xc5b2}}, nil, 3},
		{"wrong signature", fakeNativeIdentity{statusErr: errBadStatus, ids: []uint16{0, 0x1234, 0xffff}}, errBadStatus, 3},
		{"read errors", fakeNativeIdentity{statusErr: syscall.ENXIO, idErr: syscall.EIO}, syscall.ENXIO, 3},
		{"rejected controller", fakeNativeIdentity{statusErr: ErrRejected, idErr: syscall.ENXIO}, ErrRejected, 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := verifyNativeMCU(&tc.reader)
			if !errors.Is(err, tc.wantErr) || tc.reader.reads != tc.wantReads {
				t.Fatalf("verify = %v, %d reads; want %v, %d", err, tc.reader.reads, tc.wantErr, tc.wantReads)
			}
		})
	}
}

func TestLPSSFallbackNeedsEveryI801ToNAK(t *testing.T) {
	box := newAutoBox(t, "", "nak", sysOpts{})
	busy := addNativeAdapter(t, "1", "SMBus I801 adapter")
	nativeProbe = func(string, bool) error { return syscall.ENXIO }
	i801Answers = func(path string) bool { return path == busy }
	if got, err := findBus(); got != "" || err == nil {
		t.Fatalf("findBus = %q, %v", got, err)
	}
	if !box.untouched() {
		t.Fatal("mapped LPSS with an uncertain I801")
	}
}

func TestAutomaticDiscoveryPreservesForce(t *testing.T) {
	newAutoBox(t, "", "ack", sysOpts{noMCU: true})
	var forced []bool
	nativeProbe = func(_ string, force bool) error {
		forced = append(forced, force)
		if !force {
			return syscall.EBUSY
		}
		return nil
	}
	if _, err := findBusAt(controllerAddr, false); !errors.Is(err, syscall.EBUSY) {
		t.Fatalf("unforced: %v", err)
	}
	if got, err := findBusAt(controllerAddr, true); err != nil || got != filepath.Join(i2cDevDir, "i2c-0") {
		t.Fatalf("forced: %q, %v", got, err)
	}
	if !reflect.DeepEqual(forced, []bool{false, true}) {
		t.Fatalf("force flags: %v", forced)
	}
}

func TestAddressScanDoesNotRequireLEDIdentity(t *testing.T) {
	newAutoBox(t, "", "nak", sysOpts{})
	nativeProbe = func(string, bool) error { t.Fatal("address scan tried to validate the 0x3a MCU"); return syscall.ENXIO }
	paths, err := scanBuses("", false)
	if err != nil || !reflect.DeepEqual(paths, []string{filepath.Join(i2cDevDir, "i2c-0")}) {
		t.Fatalf("scanBuses: %v, %v", paths, err)
	}
}

func TestDiscoveryDoesNotRepeatSharedLockTimeout(t *testing.T) {
	newAutoBox(t, "", "ack", sysOpts{noMCU: true})
	addNativeAdapter(t, "1", dwAdapter)
	calls := 0
	nativeProbe = func(string, bool) error { calls++; return hw.ErrLockTimeout }
	if _, err := findBus(); !errors.Is(err, hw.ErrLockTimeout) || calls != 1 {
		t.Fatalf("findBus: %v, %d lock waits", err, calls)
	}
}

func TestLPSSKeepsFastNAKPath(t *testing.T) {
	box := newAutoBox(t, "", "nak", sysOpts{})
	nativeProbe = func(string, bool) error {
		t.Fatal("empty I801 entered slow status/identity retries")
		return syscall.ENXIO
	}
	if got, err := findBus(); err != nil || got != lpssPath {
		t.Fatalf("findBus: %q, %v", got, err)
	}
	if box.asked != 1 || !box.untouched() {
		t.Fatalf("I801 checks %d; memory untouched %v", box.asked, box.untouched())
	}
}
