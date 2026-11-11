package nasctl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLEDProbeRetriesAndKeepsReason(t *testing.T) {
	f := newFixture(t)
	count := filepath.Join(f.dir, "count")
	write(t, f.c.Env.LEDI2C, "#!/bin/sh\necho x >>"+count+"\nif [ $(wc -l <"+count+") -lt 3 ]; then echo 'ERROR: no ack from 0x3a' >&2; exit 1; fi\necho '{\"bus\":\"/dev/i2c-4\"}'\n")
	if err := os.Chmod(f.c.Env.LEDI2C, 0o755); err != nil {
		t.Fatal(err)
	}
	out, ok, reason := f.c.ledProbe()
	if !ok || reason != "" || !strings.Contains(out, "i2c-4") {
		t.Errorf("third try should succeed: ok %v reason %q out %q", ok, reason, out)
	}

	write(t, f.c.Env.LEDI2C, "#!/bin/sh\necho x >>"+count+"\necho 'ERROR: no ack from 0x3a' >&2\nexit 1\n")
	_ = os.Remove(count)
	if _, ok, reason := f.c.ledProbe(); ok || reason != "no ack from 0x3a" {
		t.Errorf("ok %v reason %q", ok, reason)
	}
	if data, _ := os.ReadFile(count); strings.Count(string(data), "x") != 3 {
		t.Errorf("tries: %q", data)
	}
}

func TestBuildKeepsLEDProbeReason(t *testing.T) {
	r := Build(Inputs{LEDRan: true, LEDError: "no ack from 0x3a"})
	if r.LED.Probe != "failed" || r.LED.Error != "no ack from 0x3a" {
		t.Errorf("led: %+v", r.LED)
	}
}
