package ledctl

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestUnassignedStandbySources(t *testing.T) {
	r := newRig(t, "DXP2800")
	writeSystem(r)
	r.write("devs.ini", "[dev1]\ndevice=\"sda\"\nspundown=\"1\"\n[dev2]\ndevice=\"sdc\"\nspundown=\"1\"\n[dev3]\ndevice=\"sdd\"\nspundown=\"0\"\n[dev4]\ndevice=\"sde\"\nspundown=\"bad\"\n[dev5]\ndevice=\"sdf\"\nspundown=\"1\n[dev6]\nspundown=\"1\"\n")
	r.c.disksLoad()
	for dev, want := range map[string]bool{"sda": false, "sdc": true, "sdd": false, "sde": false, "sdf": false, "missing": false} {
		if got := r.c.diskStandby(dev); got != want {
			t.Errorf("%s standby = %t, want %t", dev, got, want)
		}
	}
	r.write("devs.ini", "[dev2]\ndevice=\"sdc\"\nspundown=\"0\"\n")
	r.c.disksLoad()
	if r.c.diskStandby("sdc") {
		t.Fatal("wake did not clear standby")
	}
	if err := os.Remove(filepath.Join(r.dir, "devs.ini")); err != nil {
		t.Fatal(err)
	}
	r.c.disksLoad()
	if r.c.diskStandby("sdc") {
		t.Fatal("missing file kept stale standby")
	}
	if err := os.Remove(filepath.Join(r.dir, "disks.ini")); err != nil {
		t.Fatal(err)
	}
	r.write("devs.ini", "[dev2]\ndevice=\"sdc\"\nspundown=\"1\"\n")
	r.c.disksLoad()
	if !r.c.diskStandby("sdc") {
		t.Fatal("missing disks.ini blocked unassigned state")
	}
}

func TestUnassignedStandbyLEDAndBays(t *testing.T) {
	for _, mode := range []string{"dim", "off", "pulse", "normal"} {
		t.Run(mode, func(t *testing.T) {
			r := newRig(t, "DXP2800")
			writeSystem(r)
			r.write("disks.ini", "")
			r.write("devs.ini", "[dev1]\ndevice=\"sda\"\nspundown=\"1\"\n")
			r.config(fmt.Sprintf("disk1_color=\"1 2 3\"\nstandby_mode=%q\nstandby_level=\"20\"\n", mode))
			stop, clock := startDaemon(t, r)
			look, brightness := "on", "255"
			switch mode {
			case "dim":
				brightness = "51"
			case "off":
				look = "off"
			case "pulse":
				look = "breath"
			}
			waitFor(t, r, "unassigned standby", func() bool { s := r.chip.get("disk1"); return s[0] == look && (mode == "off" || s[1] == brightness) })
			settle(t, r, clock)
			before := countCalls(r.chip.callList(), "set ")
			settle(t, r, clock)
			if after := countCalls(r.chip.callList(), "set "); after != before {
				t.Error("unchanged standby rewrote LEDs")
			}
			r.write("devs.ini", "[dev1]\ndevice=\"sda\"\nspundown=\"0\"\n")
			waitFor(t, r, "unassigned awake", func() bool { s := r.chip.get("disk1"); return s[0] == "on" && s[1] == "255" })
			if rc := stop(); rc != 0 {
				t.Fatalf("stop: %d", rc)
			}
			r.write("devs.ini", "[dev1]\ndevice=\"sda\"\nspundown=\"1\"\n")
			r.out.Reset()
			r.c.baysJSON()
			var bays []struct {
				Standby bool   `json:"standby"`
				Slot    string `json:"slot"`
			}
			if err := json.Unmarshal(r.out.Bytes(), &bays); err != nil {
				t.Fatal(err)
			}
			if len(bays) != 2 || !bays[0].Standby || bays[0].Slot != "" {
				t.Fatalf("bays = %+v", bays)
			}
			if r.run.called("hdparm") || r.run.called("smartctl") {
				t.Fatal("queried a drive")
			}
		})
	}
}
