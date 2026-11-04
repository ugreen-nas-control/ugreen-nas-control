package nasctl

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ugreen-nas-control/ugreen-nas-control/internal/fan/conflict"
)

// RivalProc is a rival process that runs now.
type RivalProc struct {
	Name, Program, Controls string
	PID                     int
	Up                      string
}

// RivalPlugin is a known rival plugin and what of it is on the box.
type RivalPlugin struct {
	ID, Name, Controls, Version string
	Plg, WebDir, BootDir        bool
}

// RivalDiag is everything the rivals section is made of.
type RivalDiag struct {
	Procs     []RivalProc
	Plugins   []RivalPlugin // only the ones with a .plg or a folder
	Modules   []string      // "it87 (fan)"
	Hwmon     []string      // pwm outputs in manual mode
	Cron      []string      // "file: tool"
	Scripts   []string      // User Scripts that write fans or LEDs
	FanYields []string      // what the fan daemon yields to now
	FanOn     bool
	LEDBlock  string
}

func controlsText(c []string) string { return strings.Join(c, "+") }

func rivalSection(d RivalDiag) Section {
	var l []string
	if len(d.Procs) == 0 {
		l = append(l, "running now: none")
	}
	for _, p := range d.Procs {
		up := ""
		if p.Up != "" {
			up = ", up " + p.Up
		}
		l = append(l, fmt.Sprintf("RUNNING: %s (%s) pid %d%s, writes %s", p.Name, p.Program, p.PID, up, p.Controls))
	}
	for _, p := range d.Plugins {
		running := false
		for _, r := range d.Procs {
			if r.Name == p.Name {
				running = true
			}
		}
		if running {
			continue
		}
		state := "installed, not running"
		if !p.Plg {
			state = "leftover folder, no .plg"
		}
		l = append(l, fmt.Sprintf("%s: %s (plg %t, web folder %t, boot folder %t, version %s), controls %s", p.Name, state, p.Plg, p.WebDir, p.BootDir, orDash(p.Version), p.Controls))
	}
	l = append(l, "kernel modules holding our devices: "+orDash(strings.Join(d.Modules, ", ")))
	l = append(l, "hwmon pwm in manual mode (someone sets the speed): "+orDash(strings.Join(d.Hwmon, "; ")))
	l = append(l, "cron entries that run such tools: "+orDash(strings.Join(d.Cron, "; ")))
	l = append(l, "user scripts that write fans or LEDs: "+orDash(strings.Join(d.Scripts, "; ")))
	yield := "no"
	if len(d.FanYields) > 0 {
		yield = "yes, to " + strings.Join(d.FanYields, ", ")
	}
	if !d.FanOn {
		yield = "n/a (fan control is off)"
	}
	l = append(l, "UGREEN NAS Control fan daemon yields: "+yield)
	l = append(l, "LED blocker: "+orDash(d.LEDBlock))
	return Section{Title: "Other fan and LED tools", Body: strings.Join(l, "\n")}
}

// rivalChecks: CONFLICT lines for what runs now come first, then what is only installed.
func rivalChecks(d RivalDiag) []string {
	var out []string
	for _, p := range d.Procs {
		out = append(out, fmt.Sprintf("CONFLICT: %s is running (pid %d) and writes the same %s", p.Name, p.PID, strings.ReplaceAll(p.Controls, "fan", "fan chip")))
	}
	for _, s := range d.Scripts {
		out = append(out, "CONFLICT? user script writes fans or LEDs: "+s)
	}
	for _, p := range d.Plugins {
		running := false
		for _, r := range d.Procs {
			if r.Name == p.Name {
				running = true
			}
		}
		if !running && p.Plg {
			out = append(out, "rival installed, not running: "+p.Name)
		}
	}
	for _, h := range d.Hwmon {
		out = append(out, "pwm in manual mode: "+h)
	}
	return out
}

var (
	cronToolRe   = regexp.MustCompile(`(?i)fanctrl\w*|autofan|ugreen[\w-]*|pwm\d*|i2cset|it87\w*|ugreen-leds\w*`)
	scriptToolRe = regexp.MustCompile(`(?i)pwm\d*(_enable)?|i2cset|ugreen[\w-]*|it87\w*|fanctrl\w*`)
)

// cronTools names the tools a cron file runs, never its command lines.
func cronTools(dir, file string, data []byte) string {
	set := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		for _, m := range cronToolRe.FindAllString(line, -1) {
			set[strings.ToLower(m)] = true
		}
	}
	if len(set) == 0 {
		return ""
	}
	var names []string
	for n := range set {
		names = append(names, n)
	}
	sort.Strings(names)
	return filepath.Join(filepath.Base(dir), file) + ": " + strings.Join(names, ",")
}

func (c *Ctl) cronEntries() []string {
	var out []string
	dirs := []string{c.Env.CronD, filepath.Join(c.Env.PluginsBoot, "dynamix")}
	own, _ := filepath.Glob(filepath.Join(c.Env.PluginsBoot, "*", "*.cron"))
	for _, f := range own {
		dirs = append(dirs, filepath.Dir(f))
	}
	seen := map[string]bool{}
	for _, dir := range dirs {
		files, _ := filepath.Glob(filepath.Join(dir, "*"))
		for _, f := range files {
			if seen[f] || (dir != c.Env.CronD && !strings.HasSuffix(f, ".cron")) {
				continue
			}
			seen[f] = true
			// Our own cron file runs our own tool.
			if filepath.Base(filepath.Dir(f)) == "ugreen-nas-control" {
				continue
			}
			data, err := os.ReadFile(f)
			if err != nil {
				continue
			}
			if s := cronTools(dir, filepath.Base(f), data); s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

// userScripts names the User Scripts that touch fans or LEDs and says whether they are scheduled.
func (c *Ctl) userScripts() []string {
	root := filepath.Join(c.Env.PluginsBoot, "user.scripts")
	var sched map[string]struct {
		Frequency string `json:"frequency"`
	}
	if data, err := os.ReadFile(filepath.Join(root, "schedule.json")); err == nil {
		_ = json.Unmarshal(data, &sched)
	}
	dirs, _ := filepath.Glob(filepath.Join(root, "scripts", "*"))
	var out []string
	for _, d := range dirs {
		data, err := os.ReadFile(filepath.Join(d, "script"))
		if err != nil {
			continue
		}
		m := scriptToolRe.FindAllString(string(data), -1)
		if len(m) == 0 {
			continue
		}
		scheduled := false
		for k, v := range sched {
			if strings.HasPrefix(k, d+"/") && v.Frequency != "" && v.Frequency != "disabled" {
				scheduled = true
			}
		}
		out = append(out, fmt.Sprintf("%s (scheduled %t)", filepath.Base(d), scheduled))
	}
	sort.Strings(out)
	return out
}

func (c *Ctl) rivalDiag(st *fanStatus) RivalDiag {
	r := conflict.Default()
	r.PluginsBoot, r.PluginsWeb, r.ProcModules, r.Proc, r.Hwmon = c.Env.PluginsBoot, c.Env.PluginsWeb, c.Env.ProcModules, c.Env.Proc, c.Env.Hwmon
	d := RivalDiag{LEDBlock: c.ledBlocker(), FanOn: !c.fanOff()}
	for _, p := range r.Running() {
		rp := RivalProc{Name: p.Name, Program: p.ID, Controls: controlsText(p.Controls), PID: p.PID}
		if up, ok := procUptime(c.Env.Proc, p.PID); ok {
			rp.Up = up.String()
		}
		d.Procs = append(d.Procs, rp)
	}
	ver := entityRe("version")
	for _, k := range conflict.Known() {
		p := RivalPlugin{ID: k.ID, Name: k.Name, Controls: controlsText(k.Controls),
			Plg:     isFile(filepath.Join(c.Env.PluginsBoot, k.ID+".plg")),
			WebDir:  isDir(filepath.Join(c.Env.PluginsWeb, k.ID)),
			BootDir: isDir(filepath.Join(c.Env.PluginsBoot, k.ID))}
		if !p.Plg && !p.WebDir && !p.BootDir {
			continue
		}
		if data, err := os.ReadFile(filepath.Join(c.Env.PluginsBoot, k.ID+".plg")); err == nil {
			if m := ver.FindSubmatch(data); m != nil {
				p.Version = scrub(string(m[1]))
			}
		}
		d.Plugins = append(d.Plugins, p)
	}
	for _, f := range r.Detect() {
		switch f.Kind {
		case conflict.KindModule:
			d.Modules = append(d.Modules, fmt.Sprintf("%s (%s)", f.ID, controlsText(f.Controls)))
		case conflict.KindHwmon:
			d.Hwmon = append(d.Hwmon, f.ID+" "+f.Detail)
		}
	}
	d.Cron, d.Scripts = c.cronEntries(), c.userScripts()
	if st != nil && st.Daemon != nil {
		d.FanYields = st.Daemon.Conflict
	}
	return d
}
