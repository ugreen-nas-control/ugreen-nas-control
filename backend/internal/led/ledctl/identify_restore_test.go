package ledctl

import (
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type identifyRestoreChip struct {
	*memChip
	marker      string
	restore     bool
	fail        bool
	beforeWrite func()
	t           *testing.T
}

func (m *identifyRestoreChip) Exec(args []string, out, errw io.Writer) int {
	if m.restore && len(args) >= 3 && args[0] == "set" {
		if _, err := os.Stat(m.marker); err != nil {
			m.t.Error("Identify marker disappeared before restoration finished")
		}
		if m.fail && args[2] == "color" {
			return 1
		}
		if m.beforeWrite != nil {
			m.beforeWrite()
		}
	}
	rc := m.memChip.Exec(args, out, errw)
	if rc == 0 && len(args) >= 3 && args[0] == "set" && (args[2] == "off" || args[2] == "on") {
		s := m.get(args[1])
		s[5], s[6] = "0", "0"
		if args[2] == "off" {
			s[1] = "0"
		}
		m.set(args[1], s)
	}
	return rc
}

func TestIdentifyRestoresOffBrightnessAndKeepsMarkerUntilComplete(t *testing.T) {
	r := newRig(t, "DXP4800 GT")
	m := &identifyRestoreChip{memChip: r.chip, marker: r.env.IdentifyPrefix + "disk2", t: t}
	r.c.Chip = m
	before := [7]string{"off", "32", "255", "160", "0", "0", "0"}
	r.chip.set("disk2", before)
	if rc := r.ctl("identify", "disk2", "30"); rc != 0 {
		t.Fatalf("start: %d %s", rc, r.err.String())
	}
	m.restore = true
	if rc := r.ctl("identify-stop", "disk2"); rc != 0 || r.chip.get("disk2") != before {
		t.Fatalf("restore: %d %v %s", rc, r.chip.get("disk2"), r.err.String())
	}
	if _, err := os.Stat(m.marker); !os.IsNotExist(err) {
		t.Fatal("successful restore did not remove marker")
	}
}

func TestIdentifyFailedRestoreCanBeRetried(t *testing.T) {
	r := newRig(t, "DXP4800 GT")
	m := &identifyRestoreChip{memChip: r.chip, marker: r.env.IdentifyPrefix + "disk1", t: t}
	r.c.Chip = m
	before := [7]string{"on", "32", "255", "160", "0", "0", "0"}
	r.chip.set("disk1", before)
	if rc := r.ctl("identify", "disk1", "30"); rc != 0 {
		t.Fatal(rc)
	}
	m.restore, m.fail = true, true
	if rc := r.ctl("identify-stop", "disk1"); rc == 0 || !strings.Contains(r.c.identifyingLEDs(), `"disk1"`) {
		t.Fatal("failed restore lost its saved state or reported success")
	}
	m.fail = false
	if rc := r.ctl("identify-stop", "disk1"); rc != 0 || r.chip.get("disk1") != before {
		t.Fatalf("retry: %d %v", rc, r.chip.get("disk1"))
	}
}

func TestIdentifyRenewalWaitsForRestoration(t *testing.T) {
	r := newRig(t, "DXP4800 GT")
	m := &identifyRestoreChip{memChip: r.chip, marker: r.env.IdentifyPrefix + "disk1", t: t}
	r.c.Chip = m
	before := [7]string{"on", "32", "255", "160", "0", "0", "0"}
	r.chip.set("disk1", before)
	if rc := r.ctl("identify", "disk1", "30"); rc != 0 {
		t.Fatal(rc)
	}
	oldToken := r.run.spawned[0].Args[4]
	entered, resume := make(chan struct{}), make(chan struct{})
	var blocked atomic.Bool
	var resumeOnce sync.Once
	release := func() { resumeOnce.Do(func() { close(resume) }) }
	defer release()
	m.restore = true
	m.beforeWrite = func() {
		if blocked.CompareAndSwap(false, true) {
			close(entered)
			<-resume
		}
	}
	stop := New(r.env, m, r.run, io.Discard, io.Discard)
	renew := New(r.env, m, r.run, io.Discard, io.Discard)
	stopped, renewed := make(chan int, 1), make(chan int, 1)
	go func() { stopped <- stop.identifyStop("disk1", oldToken) }()
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("restoration never reached the controller")
	}
	go func() { renewed <- renew.identifyStart("disk1", "30") }()
	select {
	case rc := <-renewed:
		release()
		t.Fatalf("renewal finished during restoration: %d", rc)
	case <-time.After(30 * time.Millisecond):
	}
	release()
	for _, done := range []chan int{stopped, renewed} {
		select {
		case rc := <-done:
			if rc != 0 {
				t.Fatal(rc)
			}
		case <-time.After(time.Second):
			t.Fatal("Identify operation did not complete")
		}
	}
	if rc := stop.identifyStop("disk1", oldToken); rc != 0 || !strings.Contains(renew.identifyingLEDs(), `"disk1"`) {
		t.Fatal("old timer ended the renewed Identify")
	}
	if rc := renew.identifyStop("disk1", ""); rc != 0 || r.chip.get("disk1") != before {
		t.Fatalf("renewed snapshot was lost: rc=%d state=%v", rc, r.chip.get("disk1"))
	}
}
