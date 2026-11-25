package fan

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/ugreen-nas-control/ugreen-nas-control/internal/fan/conflict"
	"github.com/ugreen-nas-control/ugreen-nas-control/internal/hw"
)

// powerACRecordPath keeps what `power-on-ac set` last sent. The GT cannot be read back, so this is the only answer
// there; on other models it is shown next to the value read.
var powerACRecordPath = envOr("UGREEN_FAN_POWER_AC", "/boot/config/plugins/ugreen-nas-control/power_ac.json")

var errACUsage = errors.New("usage: power-on-ac [set on|off|last [--force] | restore]")

// errACUnknown refuses a set from a value this tool cannot decode: an older BIOS encodes the same bits differently,
// so writing the target bits could mean something else there. --force writes anyway.
var errACUnknown = errors.New("the board reports a value this tool does not know")

func rawACText(raw map[string]int) string {
	keys := make([]string, 0, len(raw))
	for k := range raw {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%s 0x%02x", strings.ToUpper(k), raw[k])
	}
	return strings.Join(parts, ", ")
}

type acRecord struct {
	Mode string `json:"mode"`
	Time int64  `json:"time"`
}

func readACRecord() *acRecord {
	b, err := os.ReadFile(powerACRecordPath)
	if err != nil {
		return nil
	}
	var r acRecord
	if json.Unmarshal(b, &r) != nil || r.Mode == "" {
		return nil
	}
	return &r
}

// runPowerAC is the command: no argument reads, `set <mode> [--force]` writes, `restore` writes the saved choice back when the
// board no longer holds it (the BIOS sets its own default again at every start). rival returns an error while another fan
// tool runs (only asked for a write). The port lock is held for the whole access.
func runPowerAC(args []string, model string, env *acEnv, rival func() error, at time.Time) (map[string]any, error) {
	var want acMode
	restore := false
	force := false
	if n := len(args); n > 0 && args[n-1] == "--force" {
		force, args = true, args[:n-1]
	}
	switch {
	case len(args) == 0 && !force:
	case len(args) == 1 && args[0] == "restore" && !force:
		restore = true
	case len(args) == 2 && args[0] == "set":
		m, ok := parseACMode(args[1])
		if !ok {
			return nil, errACUsage
		}
		want = m
	default:
		return nil, errACUsage
	}
	if restore && readACRecord() == nil {
		// Nothing was ever set here: do not even open the chip.
		return map[string]any{"ok": true, "model": model, "restored": false, "reason": "no saved choice"}, nil
	}
	if want != "" {
		if err := rival(); err != nil {
			return nil, err
		}
	}
	defer env.close()

	out := map[string]any{"ok": true, "model": model}
	err := withLock(func() error {
		ac, err := powerACFor(model, env)
		if err != nil {
			return err
		}
		if restore {
			reason, err := restoreAC(ac, rival, out)
			if err != nil {
				return err
			}
			out["restored"] = reason == ""
			if reason != "" {
				out["reason"] = reason
			}
		}
		if want != "" {
			if !force {
				if st, err := ac.Get(); err == nil && st.Readable && st.Mode == acUnknown {
					return fmt.Errorf("%w (%s): the encoding may differ on this BIOS, nothing was changed; `power-on-ac set %s --force` writes anyway", errACUnknown, rawACText(st.Raw), want)
				}
			}
			if err := ac.Set(want); err != nil {
				return err
			}
			if err := writeJSONAtomic(powerACRecordPath, acRecord{Mode: string(want), Time: at.Unix()}); err != nil {
				return fmt.Errorf("set, but the record could not be written: %w", err)
			}
			out["set"] = string(want)
		}
		st, err := ac.Get()
		if err != nil {
			return err
		}
		out["mode"], out["readable"], out["raw"], out["settable"], out["source"] = st.Mode, st.Readable, st.Raw, st.Settable, st.Source
		if want != "" {
			out["verified"] = st.Readable && st.Mode == want
		}
		if set, _ := out["set"].(string); restore && set != "" {
			out["verified"] = st.Readable && st.Mode == acMode(set)
		}
		if !st.Readable {
			out["note"] = "this model cannot report the setting; last_set is what this tool sent, not read back"
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if r := readACRecord(); r != nil {
		out["last_set"] = r
	} else {
		out["last_set"] = nil
	}
	return out, nil
}

// restoreAC writes the saved choice back where the board can be read and no longer holds it. A reason says why nothing was
// written. A board value this tool cannot decode is left alone (that needs `set --force` by hand).
func restoreAC(ac acLoss, rival func() error, out map[string]any) (reason string, err error) {
	rec := readACRecord()
	if rec == nil {
		return "no saved choice", nil
	}
	m, ok := parseACMode(rec.Mode)
	if !ok {
		return "the saved choice is not a mode", nil
	}
	st, err := ac.Get()
	if err != nil {
		return "", err
	}
	switch {
	case !st.Readable:
		return "this model cannot report the setting, so a differing value cannot be told", nil
	case st.Mode == m:
		return "the board already holds it", nil
	case st.Mode == acUnknown:
		return "the board reports a value this tool does not know (" + rawACText(st.Raw) + ")", nil
	case !slices.Contains(st.Settable, m):
		return "this model cannot be set to " + string(m), nil
	}
	if err := rival(); err != nil {
		return "", err
	}
	out["set"] = string(m)
	return "", ac.Set(m)
}

// PowerACCommand is `ugreen-nas-fan power-on-ac [set on|off|last [--force]]`.
func PowerACCommand(args []string) {
	env := &acEnv{openPorts: hw.OpenPorts, openMem: hw.OpenMem}
	rival := func() error {
		if r := conflict.Blocking(detectConflicts(), conflict.Fan); len(r) > 0 {
			return fmt.Errorf("%s controls the fans too; switch one of the two off first", rivalNames(r))
		}
		return nil
	}
	out, err := runPowerAC(args, productName(), env, rival, time.Now())
	if errors.Is(err, errACUsage) {
		usage()
	}
	if err != nil {
		printJSON(map[string]any{"ok": false, "error": err.Error()})
		os.Exit(1)
	}
	printJSON(out)
}
