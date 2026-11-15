package fan

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ugreen-nas-control/ugreen-nas-control/internal/hw"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "fan-ec-lock-")
	if err != nil {
		panic(err)
	}
	restore := hw.SetECLockPath(filepath.Join(dir, "ec.lock"))
	code := m.Run()
	restore()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func TestECTransactionsWaitForExternalLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ug-ec.lock")
	defer hw.SetECLockPath(path)()
	old := hw.LockTimeout
	hw.LockTimeout = 20 * time.Millisecond
	t.Cleanup(func() { hw.LockTimeout = old })
	p := newFakeEC()
	c := &EC{ports: p, spec: ecSpecGT}
	if err := hw.WithLockAt(path, func() error {
		if _, err := c.read(0xb0); !errors.Is(err, hw.ErrLockTimeout) {
			t.Errorf("read: %v", err)
		}
		if err := c.write(0xb1, 60); !errors.Is(err, hw.ErrLockTimeout) {
			t.Errorf("write: %v", err)
		}
		if err := c.SendCommand(ecCmdACOff); !errors.Is(err, hw.ErrLockTimeout) {
			t.Errorf("command: %v", err)
		}
		if p.state != 0 || len(p.writes) != 0 {
			t.Fatal("accessed EC while external lock was held")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.read(0xb0); err != nil {
		t.Fatalf("did not resume after external unlock: %v", err)
	}
}
