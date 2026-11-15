//go:build linux

package chip

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/ugreen-nas-control/ugreen-nas-control/internal/hw"
)

const (
	ioctlSlave      = 0x0703
	ioctlSlaveForce = 0x0706
	ioctlSMBus      = 0x0720

	smbusWrite = 0
	smbusRead  = 1

	smbusByte     = 1
	smbusByteData = 2
	smbusWordData = 3
	smbusBlockMax = 32
)

// lockFile is replaced in tests.
var lockFile = "/var/run/ugreen-led-i2c.lock"

// struct i2c_smbus_ioctl_data: u8 read_write, u8 command, u32 size, union i2c_smbus_data *data.
type smbusIoctlData struct {
	readWrite uint8
	command   uint8
	size      uint32
	data      *[smbusBlockMax + 2]byte
}

type bus struct {
	path string
	file *os.File
	lock *os.File
	dw   *dwI2C // DXP4800 GT without a kernel driver: the controller is driven through /dev/mem
	gt   bool   // DXP4800 GT: SMBus block writes instead of I2C block writes
}

func ioctl(fd, request, arg uintptr) error {
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, fd, request, arg); errno != 0 {
		return errno
	}
	return nil
}

// nativeProbe is read-only and replaced in tests. Only an explicit --force permits an address owned by another driver.
var nativeProbe = probeNativeMCU

// findBus validates all native candidates before considering a /dev/mem fallback.
// DXP480T's separate power MCU uses findBusAt with its own address and no 0x3a probe.
func findBus() (string, error) { return findBusAt(controllerAddr, false) }

func findBusAt(addr uintptr, force bool) (string, error) {
	paths := nativeCandidates()
	var failures []error
	var i801 []string
	canLPSS := addr == controllerAddr && lpssAuto()
	i801NAK := map[string]bool{}
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			failures = append(failures, fmt.Errorf("%s is missing; load the i2c-dev module (modprobe i2c-dev): %w", path, err))
			continue
		}
		if addr != controllerAddr {
			return path, nil
		}
		name, _ := os.ReadFile(filepath.Join(i2cSysBus, filepath.Base(path), "name"))
		if strings.HasPrefix(string(name), "SMBus I801 adapter") {
			i801 = append(i801, path)
			// Preserve the existing fast NAK check on LPSS boards. Register retries on an
			// empty I801 would otherwise add hundreds of milliseconds to every LED write.
			if canLPSS && !i801Answers(path) {
				i801NAK[path] = true
				failures = append(failures, fmt.Errorf("%s: %w", path, syscall.ENXIO))
				continue
			}
		}
		err := nativeProbe(path, force)
		if err == nil {
			return path, nil
		}
		if errors.Is(err, hw.ErrLockTimeout) {
			return "", err // the lock is shared by every adapter; another probe would wait again
		}
		failures = append(failures, fmt.Errorf("%s: %w", path, err))
	}

	// A native adapter means the kernel owns that controller: don't bypass it through /dev/mem.
	// The LPSS fallback keeps its existing ACPI/ownership checks and only follows a definite I801 NAK.
	if len(paths) == 0 {
		if names, _ := dwControllers(); len(names) > 0 {
			return dwPath, nil
		}
	}
	if canLPSS {
		allNAK := len(i801) > 0
		for _, path := range i801 {
			if !i801NAK[path] {
				allNAK = false // busy, timed out or responding: never bypass it on a doubt
			}
		}
		if len(paths) == 0 || allNAK {
			return lpssPath, nil
		}
	}
	if len(failures) > 0 {
		return "", fmt.Errorf("no LED MCU found on native I2C adapters: %w", errors.Join(failures...))
	}
	return "", errors.New("no SMBus I801 or DesignWare I2C adapter found; load the i2c_i801 module (modprobe i2c_i801) and check dmesg | grep -i -e i801 -e designware")
}

// nativeCandidates prefers the HT32 ACPI adapter, then I801, then DesignWare, each in numeric order.
// Display and unrelated buses are never probed. Evidence: miskcoo PRs #115 and #126.
func nativeCandidates() []string {
	names, _ := filepath.Glob(filepath.Join(i2cSysBus, "i2c-*", "name"))
	sort.Slice(names, func(i, j int) bool {
		return adapterNumber(names[i]) < adapterNumber(names[j])
	})
	acpi := map[string]bool{}
	devices, _ := filepath.Glob(filepath.Join(i2cSysBus, "i2c-HT32*"))
	for _, dev := range devices {
		resolved, err := filepath.EvalSymlinks(dev)
		if err == nil {
			acpi[filepath.Base(filepath.Dir(resolved))] = true
		}
	}
	var paths []string
	for rank := range 3 {
		for _, name := range names {
			content, _ := os.ReadFile(name)
			bus := filepath.Base(filepath.Dir(name))
			i801 := strings.HasPrefix(string(content), "SMBus I801 adapter")
			dw := strings.HasPrefix(string(content), dwAdapter)
			if !i801 && !dw {
				continue
			}
			r := 2
			if acpi[bus] {
				r = 0
			} else if i801 {
				r = 1
			}
			if r == rank {
				paths = append(paths, filepath.Join(i2cDevDir, bus))
			}
		}
	}
	return paths
}

func adapterNumber(name string) int {
	n, _ := strconv.Atoi(strings.TrimPrefix(filepath.Base(filepath.Dir(name)), "i2c-"))
	return n
}

func probeNativeMCU(path string, force bool) error {
	lock, err := acquireLock()
	if err != nil {
		return err
	}
	b, err := openNativeBus(path, controllerAddr, force, lock)
	if err != nil {
		return err
	}
	defer b.close()
	return verifyNativeMCU(b)
}

type nativeIdentityReader interface {
	status(string) (ledStatus, error)
	readWord(byte) (uint16, error)
}

// verifyNativeMCU has read operations only; discovery never tests with a LED command.
func verifyNativeMCU(b nativeIdentityReader) error {
	_, statusErr := b.status("power") // includes checksum validation and bounded read retries
	if statusErr == nil {
		return nil
	}
	// Some firmware exposes the identity before it can return a valid status.
	// The identity validates the MCU, never the write protocol (upstream PR #100).
	for attempt := range 3 {
		if attempt > 0 {
			time.Sleep(100 * time.Millisecond)
		}
		if id, err := b.readWord(0x5a); err == nil && id == 0xc5b2 {
			return nil
		}
	}
	return statusErr
}

func (b *bus) readWord(register byte) (uint16, error) {
	var data [smbusBlockMax + 2]byte
	err := b.smbus(smbusRead, register, smbusWordData, &data)
	return uint16(data[0]) | uint16(data[1])<<8, err
}

// acquireLock allows one writer at a time: the web UI and the status daemon share the controller. It waits at most
// hw.LockTimeout, then fails with hw.ErrLockTimeout.
func acquireLock() (*os.File, error) {
	lock, err := os.OpenFile(lockFile, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := hw.FlockExclusive(int(lock.Fd())); err != nil {
		_ = lock.Close()
		return nil, err
	}
	return lock, nil
}

func openBus(path string, force bool) (*bus, error) {
	return openBusAt(path, controllerAddr, force)
}

func openBusAt(path string, addr uintptr, force bool) (*bus, error) {
	auto := path == ""
	if auto {
		found, err := findBusAt(addr, force)
		if err != nil {
			return nil, err
		}
		path = found
	}

	lock, err := acquireLock()
	if err != nil {
		return nil, err
	}

	if path == lpssPath {
		if lpssOff() {
			_ = lock.Close()
			return nil, errLPSSOff
		}
		dw, err := openLPSS(&lpssReport{}, uint8(addr), true)
		if err != nil {
			_ = lock.Close()
			if auto && lpssNoteRefused(err) {
				return openBusAt("", addr, force) // the old behaviour; findBus skips the LPSS path now
			}
			return nil, err
		}
		lpssNoteOK()
		return &bus{path: path, dw: dw, lock: lock}, nil
	}

	if path == dwPath {
		dw, err := openDW(uint8(addr))
		if err != nil {
			_ = lock.Close()
			return nil, err
		}
		return &bus{path: path, dw: dw, lock: lock, gt: true}, nil
	}

	return openNativeBus(path, addr, force, lock)
}

// openNativeBus takes ownership of lock, including on failure.
func openNativeBus(path string, addr uintptr, force bool, lock *os.File) (*bus, error) {
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		_ = lock.Close()
		return nil, err
	}

	request := uintptr(ioctlSlave)
	if force {
		request = ioctlSlaveForce
	}

	if err := ioctl(file.Fd(), request, addr); err != nil {
		_ = file.Close()
		_ = lock.Close()
		if errors.Is(err, syscall.EBUSY) {
			return nil, fmt.Errorf("address 0x%02x on %s is used by a kernel driver (led_ugreen loaded?); unload it or pass --force", addr, path)
		}
		return nil, err
	}

	return &bus{path: path, file: file, lock: lock}, nil
}

func (b *bus) close() {
	if b.dw != nil {
		b.dw.close()
	} else {
		_ = b.file.Close()
	}
	_ = b.lock.Close()
}

func smbusIoctl(file *os.File, readWrite, command byte, size uint32, data *[smbusBlockMax + 2]byte) error {
	args := smbusIoctlData{readWrite: readWrite, command: command, size: size, data: data}
	err := ioctl(file.Fd(), ioctlSMBus, uintptr(unsafe.Pointer(&args))) //nolint:gosec // ioctl argument struct
	runtime.KeepAlive(data)
	return err
}

func (b *bus) smbus(readWrite, command byte, size uint32, data *[smbusBlockMax + 2]byte) error {
	if b.dw != nil {
		return b.dw.smbus(readWrite, command, size, data)
	}
	return smbusIoctl(b.file, readWrite, command, size, data)
}

func (b *bus) readBlock(command byte, length int) ([]byte, error) {
	var data [smbusBlockMax + 2]byte
	data[0] = byte(length)

	if err := b.smbus(smbusRead, command, smbusI2CBlockData, &data); err != nil {
		return nil, err
	}
	return append([]byte(nil), data[1:1+length]...), nil
}

func (b *bus) writeBlock(command byte, size uint32, payload []byte) error {
	var data [smbusBlockMax + 2]byte
	data[0] = byte(len(payload))
	copy(data[1:], payload)

	return b.smbus(smbusWrite, command, size, &data)
}

func (b *bus) readByte(command byte) (byte, error) {
	var data [smbusBlockMax + 2]byte

	if err := b.smbus(smbusRead, command, smbusByteData, &data); err != nil {
		return 0, err
	}
	return data[0], nil
}

// change sends one command and waits until the controller confirms it (see framesFor for the frame order).
func (b *bus) change(id, command byte, params [4]byte) error {
	return asRejected(sendFrames(b, framesFor(b.gt), id, command, params), b.detect)
}

// detect reads one byte from the controller address, as i2cdetect does; the DesignWare path has no such transfer.
func (b *bus) detect() error {
	if b.dw != nil {
		return errors.New("no detect transfer on this adapter")
	}

	var data [smbusBlockMax + 2]byte
	return smbusIoctl(b.file, smbusRead, 0, smbusByte, &data)
}

func (b *bus) status(name string) (ledStatus, error) {
	id, err := ledID(name)
	if err != nil {
		return ledStatus{}, err
	}

	busOnly := true

	for i := range retries {
		backoff(i)

		var block []byte
		if block, err = b.readBlock(regStatusBase+id, statusLength); err != nil {
			continue
		}
		busOnly = false
		var s ledStatus
		if s, err = parseStatus(name, block); err == nil {
			return s, nil
		}
	}

	if busOnly {
		err = asRejected(busFailure{err}, b.detect)
	}

	return ledStatus{}, fmt.Errorf("%s: %w", name, err)
}

// writeByteData is "i2cset -y <bus> <addr> <register> <value> b".
func (b *bus) writeByteData(register, value byte) error {
	var data [smbusBlockMax + 2]byte
	data[0] = value

	return b.smbus(smbusWrite, register, smbusByteData, &data)
}

// scanBuses lists the I2C buses to scan: the named one, or with all every /dev/i2c-N.
func scanBuses(path string, all bool) ([]string, error) {
	if all {
		paths, _ := filepath.Glob("/dev/i2c-*")
		if len(paths) == 0 {
			return nil, errors.New("no /dev/i2c-* found; load the i2c-dev module (modprobe i2c-dev)")
		}
		return paths, nil
	}
	if path == "" {
		// A scan discovers addresses, including power-only MCUs at 0x26; it must not
		// require a working LED controller at 0x3a before it can list them.
		found, err := findBusAt(0, false)
		if err != nil {
			return nil, err
		}
		path = found
	}
	return []string{path}, nil
}

// scanBus reads one byte from every address 0x03..0x77 (like "i2cdetect -y -r") and
// returns the ones that answer; busy are addresses a kernel driver already holds.
// It only reads, nothing is written to any device.
func scanBus(path string) (found, busy []string, err error) {
	lock, err := acquireLock()
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = lock.Close() }()

	if path == dwPath {
		return scanDW()
	}
	if path == lpssPath {
		return nil, nil, errors.New("scan is not supported on the LPSS controller; use lpss-probe")
	}

	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = file.Close() }()

	for addr := uintptr(0x03); addr <= 0x77; addr++ {
		name := fmt.Sprintf("0x%02x", addr)
		if err := ioctl(file.Fd(), ioctlSlave, addr); err != nil {
			if errors.Is(err, syscall.EBUSY) {
				busy = append(busy, name)
			}
			continue
		}

		var data [smbusBlockMax + 2]byte
		if smbusIoctl(file, smbusRead, 0, smbusByte, &data) == nil {
			found = append(found, name)
		}
	}

	return found, busy, nil
}

func (b *bus) busPath() string { return b.path }
