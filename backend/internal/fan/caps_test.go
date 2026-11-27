package fan

import "testing"

func TestCapsFor(t *testing.T) {
	isolate(t, "DXP4800 Plus")
	for _, tc := range []struct {
		model string
		want  FanCaps
		ok    bool
	}{
		{"DXP4800 GT", FanCaps{KindFixed, 2, true, false, 0}, true},
		{"iDX6011 Pro", FanCaps{KindFixed, 4, true, true, 0}, true},
		{"iDX6012 Pro", FanCaps{}, false},
		{"DXP2800 GT", FanCaps{KindProbe, 0, false, false, probeFloorPct}, true},
		{"DXP6800 Pro", FanCaps{KindFixed, 3, true, false, 0}, true},
		{"DXP4800 Plus", FanCaps{KindProbe, 0, true, false, probeFloorPct}, true},
		{"iDX6011", FanCaps{KindReadOnly, 2, false, false, 0}, true},
		{"iDX6012", FanCaps{KindReadOnly, 2, false, false, 0}, true},
		{"DX4600 Pro", FanCaps{}, false},
	} {
		got, ok := CapsFor(tc.model)
		if ok != tc.ok || got != tc.want {
			t.Errorf("CapsFor(%q) = %+v, %v; want %+v, %v", tc.model, got, ok, tc.want, tc.ok)
		}
	}
}

func TestStepsFromSkipsStepsBelowTheFloor(t *testing.T) {
	if got := stepsFrom(0); len(got) != len(calibSteps) {
		t.Fatalf("no floor: %v", got)
	}
	if got := stepsFrom(csFloorPct); got[0] != csFloorPct || len(got) != len(calibSteps)-3 {
		t.Fatalf("floor %d: %v", csFloorPct, got)
	}
}

// Settings, targets and calibration read the same floor the chip clamps to, on every probe model.
func TestMinPctFloorMatchesTheChipClampOfEveryProbeModel(t *testing.T) {
	for _, m := range []string{"DXP2800", "DXP4800", "DXP4800 Plus", "DXP4800 Pro", "DXP6800", "DXP6800 Plus", "DXP8800", "DXP8800 Pro", "DXP8800 Plus", "DXP2800 GT"} {
		if FanKind(m) != KindProbe || minPctFloor(m) != probeFloorPct {
			t.Errorf("%s: kind %q floor %d, want probe and %d", m, FanKind(m), minPctFloor(m), probeFloorPct)
		}
	}
	for _, m := range []string{"DXP4800 GT", "DXP6800 Pro", "iDX6011 Pro", "iDX6011"} {
		if minPctFloor(m) != 0 {
			t.Errorf("%s: floor %d, want none", m, minPctFloor(m))
		}
	}
}
