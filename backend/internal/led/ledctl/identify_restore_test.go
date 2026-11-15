package ledctl

import (
	"io"
	"os"
	"strings"
	"testing"
)

type identifyRestoreChip struct {
	*memChip
	marker  string
	restore bool
	fail    bool
	t       *testing.T
}

func (m *identifyRestoreChip) Exec(args []string, out, errw io.Writer) int {
	if m.restore && len(args) >= 3 && args[0] == "set" {
		if _, err := os.Stat(m.marker); err != nil {
			m.t.Error("Identify marker disappeared before restoration finished")
		}
		if m.fail && args[2] == "color" {
			return 1
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
