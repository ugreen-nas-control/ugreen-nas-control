package chip_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ugreen-nas-control/ugreen-nas-control/internal/led/chip"
	"github.com/ugreen-nas-control/ugreen-nas-control/internal/led/ledctl"
)

type timerRunner struct{ timers []ledctl.SpawnSpec }

func (*timerRunner) Output(string, ...string) ([]byte, int) { return nil, 1 }
func (*timerRunner) Has(string) bool                        { return false }
func (r *timerRunner) Spawn(spec ledctl.SpawnSpec) (int, error) {
	r.timers = append(r.timers, spec)
	return 1, nil
}

func gtControl(t *testing.T) (*chip.GTTestController, *ledctl.Ctl, *timerRunner, *bytes.Buffer) {
	t.Helper()
	bus, restore := chip.UseGTTestController()
	t.Cleanup(restore)
	root := t.TempDir()
	env := ledctl.EnvFromGetenv(func(key string) string {
		if key == "UGREEN_PRODUCT" {
			return "DXP4800 GT"
		}
		return filepath.Join(root, key)
	}, filepath.Join(root, "ugreen-nas"))
	env.SelfArgs = []string{"led-ctl"}
	runner, errw := &timerRunner{}, &bytes.Buffer{}
	ctl := ledctl.New(env, ledctl.InProcChip{Product: "DXP4800 GT"}, runner, &bytes.Buffer{}, errw)
	return bus, ctl, runner, errw
}

func TestGTIdentifyThroughControllerAndChip(t *testing.T) {
	bus, ctl, runner, errw := gtControl(t)
	run := func(args ...string) {
		t.Helper()
		if rc := ctl.Main(context.Background(), args); rc != 0 {
			t.Fatalf("%v: rc=%d %s", args, rc, errw.String())
		}
	}
	for bay := 1; bay <= 4; bay++ {
		name := fmt.Sprintf("disk%d", bay)
		for _, mode := range []string{"on", "off", "blink", "breath"} {
			for _, brightness := range []int{0, 20, 32, 204, 255} {
				before := chip.TestLEDState{Name: name, Mode: mode, Brightness: brightness, Color: "255 160 0"}
				if mode == "blink" || mode == "breath" {
					before.OnMs, before.OffMs = 1200, 900
				}
				bus.Set(before)
				bus.DropWrites(1) // acknowledged colour write that the MCU does not apply
				run("identify", name, "30")
				active := bus.State(name)
				if active.Mode != "blink" || active.Brightness != 255 || active.Color != "255 255 255" || active.OnMs != 100 || active.OffMs != 100 {
					t.Fatalf("Identify %s from %s/%d: %+v", name, mode, brightness, active)
				}
				// Execute the action captured from the real timer request, including its token.
				timer := runner.timers[len(runner.timers)-1].Args
				run(timer[len(timer)-3:]...)
				if got := bus.State(name); got != before {
					t.Fatalf("restored %+v, want %+v", got, before)
				}
			}
		}
	}
}

func TestGTFailedIdentifyRestoreThroughChipCanRetry(t *testing.T) {
	bus, ctl, _, errw := gtControl(t)
	before := chip.TestLEDState{Name: "disk1", Mode: "on", Brightness: 32, Color: "255 160 0"}
	bus.Set(before)
	if rc := ctl.Main(context.Background(), []string{"identify", "disk1", "30"}); rc != 0 {
		t.Fatal(rc, errw.String())
	}
	bus.DropWrites(5)
	if rc := ctl.Main(context.Background(), []string{"identify-stop", "disk1"}); rc == 0 {
		t.Fatal("accepted an acknowledged but unapplied restoration")
	}
	if _, err := os.Stat(ctl.Env.IdentifyPrefix + "disk1"); err != nil {
		t.Fatal("lost retry snapshot:", err)
	}
	if rc := ctl.Main(context.Background(), []string{"identify-stop", "disk1"}); rc != 0 || bus.State("disk1") != before {
		t.Fatalf("retry: rc=%d state=%+v %s", rc, bus.State("disk1"), errw.String())
	}
}
