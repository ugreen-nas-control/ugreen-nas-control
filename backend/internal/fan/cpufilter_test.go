package fan

import (
	"slices"
	"testing"
)

func feed(f *cpuFilter, crit int, temps ...int) []int {
	var out []int
	for _, t := range temps {
		out = append(out, *f.next(&t, crit))
	}
	return out
}

func TestCPUFilterSingleSpikeIgnored(t *testing.T) {
	var f cpuFilter
	got := feed(&f, 90, 47, 47, 93, 47, 47)
	for i, v := range got {
		if v >= 90 {
			t.Errorf("poll %d: spike passed through (%d)", i, v)
		}
	}
	d := Decide(LoadConfig("/nonexistent"), Sensors{CPU: ip(got[2])})
	if d.Failsafe != "" {
		t.Errorf("failsafe %q", d.Failsafe)
	}
}

func TestCPUFilterSustainedTriggersAndRecovers(t *testing.T) {
	var f cpuFilter
	got := feed(&f, 90, 47, 93, 93, 93, 47, 47, 47)
	want := []int{47, 47, 93, 93, 93, 47, 47}
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestCPUFilterRunawayAndUnreadable(t *testing.T) {
	var f cpuFilter
	if got := feed(&f, 90, 47, 47, 100); got[2] != 100 {
		t.Errorf("runaway delayed: %v", got)
	}
	if f.next(nil, 90) != nil {
		t.Error("unreadable must stay nil")
	}
	if got := feed(&f, 90, 92); got[0] != 92 {
		t.Errorf("first reading after unreadable: %v", got)
	}
}
