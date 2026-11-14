package panel

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ugreen-nas-control/ugreen-nas-control/internal/hw"
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "panel-ec-lock-")
	if err != nil {
		panic(err)
	}
	restore := hw.SetECLockPath(filepath.Join(dir, "ec.lock"))
	code := m.Run()
	restore()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func TestECBeeperWaitsForExternalLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ug-ec.lock")
	defer hw.SetECLockPath(path)()
	old := hw.LockTimeout
	hw.LockTimeout = 10 * time.Millisecond
	t.Cleanup(func() { hw.LockTimeout = old })
	p := &fakeEC{}
	b := &ecBeeper{p: p, reg: 0xa5, wait: time.Second, lock: noLock}
	if err := hw.WithLockAt(path, func() error {
		if err := b.On(0); !errors.Is(err, hw.ErrLockTimeout) {
			t.Errorf("beep: %v", err)
		}
		if len(p.writes) != 0 {
			t.Fatal("beeper wrote while external lock was held")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := b.On(0); err != nil {
		t.Fatalf("did not resume after unlock: %v", err)
	}
}
