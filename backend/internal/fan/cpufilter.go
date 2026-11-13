package fan

import "slices"

const (
	// cpuWindow readings (2 s apart) are taken as a median: a spike of one poll never counts, two in a row do.
	cpuWindow = 3
	// A reading this far over the critical limit is acted on at once.
	cpuRunaway = 10
)

// cpuFilter turns the raw CPU temperature into the one the curve and the emergency check use. coretemp's hottest
// core can burst 40 °C above the rest for under a second.
type cpuFilter struct{ last []int }

// next adds a reading and returns the median of the last cpuWindow ones (the lower middle with two). An unreadable
// CPU (nil) is passed on as it is and restarts the window; a reading of crit+cpuRunaway or more is passed on as is.
func (f *cpuFilter) next(raw *int, crit int) *int {
	if raw == nil {
		f.last = nil
		return nil
	}
	f.last = append(f.last, *raw)
	if len(f.last) > cpuWindow {
		f.last = f.last[1:]
	}
	if *raw >= crit+cpuRunaway {
		return raw
	}
	sorted := slices.Clone(f.last)
	slices.Sort(sorted)
	m := sorted[(len(sorted)-1)/2]
	return &m
}
