package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// The publisher: sends the status of this NAS to an MQTT broker for displays and Home Assistant, and, only
// if switched on, carries out a few fixed commands (restart, shut down, update a plugin). It only reads
// the hardware the way `status` does and never writes a fan register.
//
// Topics, below ugreen-nas/<name>/: state, notifications and updates (JSON, retained), availability
// (online/offline, the last will), cmd (commands in) and cmd_result (answers out).

var (
	notifyDir    = envOr("UGREEN_NOTIFY_DIR", "/tmp/notifications/unread")
	pluginsBoot  = envOr("UGREEN_PLUGINS_BOOT", "/boot/config/plugins")
	pluginsFresh = envOr("UGREEN_PLUGINS_FRESH", "/tmp/plugins")
	dockerStatus = envOr("UGREEN_DOCKER_STATUS", "/var/lib/docker/unraid-update-status.json")
	unraidCheck  = envOr("UGREEN_UNRAID_CHECK", "/tmp/unraidcheck/result.json")
	procNetDev   = envOr("UGREEN_PROC_NET_DEV", "/proc/net/dev")
	powerdownBin = envOr("UGREEN_POWERDOWN", "/usr/local/sbin/powerdown")
	pluginBin    = envOr("UGREEN_PLUGIN_BIN", "/usr/local/sbin/plugin")
	dockerBin    = envOr("UGREEN_DOCKER_BIN", "/usr/bin/docker")
	// Unraid's own script for pulling a new image and recreating a container, the same one its Docker page uses.
	updateContainerBin = envOr("UGREEN_UPDATE_CONTAINER", "/usr/local/emhttp/plugins/dynamix.docker.manager/scripts/update_container")
)

// MQTTConfig is read from the same fan.cfg as the fan settings.
type MQTTConfig struct {
	Enabled bool
	Host    string
	Port    int
	TLS     bool
	User    string
	Pass    string
	// Name is the part of the topics that tells this NAS from another one.
	Name     string
	Interval time.Duration
	// Control: restart and shut down. ControlUpdates: update plugins. ControlContainers: update containers.
	// All off unless switched on.
	Control           bool
	ControlUpdates    bool
	ControlContainers bool
}

var nameRe = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

func LoadMQTTConfig(path string) MQTTConfig {
	v := readValues(path)
	c := MQTTConfig{
		Enabled:           v["mqtt_enabled"] == "1",
		Host:              v["mqtt_host"],
		Port:              1883,
		TLS:               v["mqtt_tls"] == "1",
		User:              v["mqtt_user"],
		Pass:              v["mqtt_pass"],
		Name:              nameRe.ReplaceAllString(v["mqtt_name"], ""),
		Interval:          5 * time.Second,
		Control:           v["mqtt_control"] == "1",
		ControlUpdates:    v["mqtt_control_updates"] == "1",
		ControlContainers: v["mqtt_control_containers"] == "1",
	}
	if c.TLS {
		c.Port = 8883
	}
	if n, err := strconv.Atoi(v["mqtt_port"]); err == nil && n > 0 && n < 65536 {
		c.Port = n
	}
	if n, err := strconv.Atoi(v["mqtt_interval"]); err == nil && n >= 2 && n <= 60 {
		c.Interval = time.Duration(n) * time.Second
	}
	if c.Name == "" {
		h, _ := os.Hostname()
		c.Name = nameRe.ReplaceAllString(strings.ToLower(h), "")
	}
	if c.Name == "" {
		c.Name = "nas"
	}
	return c
}

// ---- What goes out ----

type PubFan struct {
	ID    string `json:"id"`
	Role  string `json:"role"`
	RPM   int    `json:"rpm"`
	Pct   *int   `json:"pct"`
	Auto  bool   `json:"auto"`
	Error string `json:"error,omitempty"`
}

type PubNVMe struct {
	Name string `json:"name"`
	Slot string `json:"slot,omitempty"`
	Temp *int   `json:"temp"`
}

type PubNet struct {
	Name      string `json:"name"`
	Link      bool   `json:"link"`
	SpeedMbit int    `json:"speed_mbit"`
	MaxMbit   int    `json:"max_mbit"`
	RxBps     int64  `json:"rx_bps"`
	TxBps     int64  `json:"tx_bps"`
}

type Alarm struct {
	Level string `json:"level"`
	Code  string `json:"code"` // cpu_hot, cpu_unreadable, disk_hot, nvme_hot, fan_stalled: for a display to translate
	Bay   int    `json:"bay,omitempty"`
	Text  string `json:"text"`
	Value *int   `json:"value,omitempty"`
	Limit int    `json:"limit,omitempty"`
	Hint  string `json:"hint,omitempty"`
}

type PubState struct {
	Time       int64       `json:"time"`
	Model      string      `json:"model"`
	Unraid     string      `json:"unraid"`
	Uptime     int         `json:"uptime"`
	Mode       string      `json:"mode"`
	FanControl bool        `json:"fan_control"`
	CPU        *int        `json:"cpu"`
	Board      *int        `json:"board"`
	CaseTemp   *int        `json:"case_temp"`
	Fans       []PubFan    `json:"fans"`
	Bays       []Bay       `json:"bays"`
	NVMe       []PubNVMe   `json:"nvme"`
	Net        []PubNet    `json:"net"`
	Storage    *PubStorage `json:"storage"`
	Parity     *PubParity  `json:"parity"`
	Alarms     []Alarm     `json:"alarms"`
}

type Notification struct {
	Level string `json:"level"`
	Title string `json:"title"`
	Text  string `json:"text"`
	Time  string `json:"time"`
}

type Update struct {
	Kind string `json:"kind"` // plugin, container, system
	Name string `json:"name"`
	File string `json:"file,omitempty"`
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
}

// ---- Notifications ----

// readNotifications reads Unraid's unread notifications (files of key="value" lines), newest first.
func readNotifications(dir string, now time.Time) []Notification {
	files, _ := filepath.Glob(filepath.Join(dir, "*.notify"))
	type item struct {
		ts int64
		n  Notification
	}
	var items []item
	for _, f := range files {
		v := readValues(f)
		ts, _ := strconv.ParseInt(v["timestamp"], 10, 64)
		level := v["importance"]
		if level == "" {
			level = "normal"
		}
		t := time.Unix(ts, 0)
		label := t.Format("15:04")
		if t.YearDay() != now.YearDay() || t.Year() != now.Year() {
			label = t.Format("02.01. 15:04")
		}
		title := v["subject"]
		if title == "" {
			title = v["event"]
		}
		items = append(items, item{ts, Notification{Level: level, Title: title, Text: v["description"], Time: label}})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ts > items[j].ts })
	out := []Notification{}
	for i, it := range items {
		if i == 20 {
			break
		}
		out = append(out, it.n)
	}
	return out
}

// ---- Updates ----

var versionRe = regexp.MustCompile(`<!ENTITY\s+version\s+"([^"]+)"`)

func plgVersion(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	if m := versionRe.FindSubmatch(b); m != nil {
		return string(m[1])
	}
	return ""
}

// versionLess compares versions such as 7.3.1 or 2026.10.26a part by part, numbers as numbers.
func versionLess(a, b string) bool {
	pa, pb := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y string
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		nx, ex := strconv.Atoi(x)
		ny, ey := strconv.Atoi(y)
		if ex == nil && ey == nil {
			if nx != ny {
				return nx < ny
			}
			continue
		}
		if x != y {
			return x < y
		}
	}
	return false
}

// readOSUpdate reads the result of Unraid's own check for a new Unraid OS (isNewer), which it keeps in
// /tmp/unraidcheck/result.json.
func readOSUpdate(path string) (Update, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Update{}, false
	}
	var r struct {
		Version string `json:"version"`
		IsNewer bool   `json:"isNewer"`
		Params  struct {
			Current string `json:"current_version"`
		} `json:"params"`
	}
	if json.Unmarshal(b, &r) != nil || !r.IsNewer || r.Version == "" {
		return Update{}, false
	}
	return Update{Kind: "system", Name: "Unraid OS", From: r.Params.Current, To: r.Version}, true
}

// readUpdates finds the updates Unraid already knows of: a plugin whose freshly downloaded copy (in /tmp/plugins,
// put there by Unraid's own check) has a higher version than the installed one, and the containers Unraid marked
// as outdated. It checks nothing itself and downloads nothing.
func readUpdates(boot, fresh, dockerFile, osFile string) []Update {
	out := []Update{}
	osKnown := false
	if u, ok := readOSUpdate(osFile); ok {
		out, osKnown = append(out, u), true
	}
	files, _ := filepath.Glob(filepath.Join(boot, "*.plg"))
	sort.Strings(files)
	for _, f := range files {
		base := filepath.Base(f)
		local, remote := plgVersion(f), plgVersion(filepath.Join(fresh, base))
		if local == "" || remote == "" || !versionLess(local, remote) {
			continue
		}
		name := strings.TrimSuffix(base, ".plg")
		kind := "plugin"
		if name == "unRAIDServer" {
			if osKnown {
				continue
			}
			kind, name = "system", "Unraid OS"
		}
		out = append(out, Update{Kind: kind, Name: name, File: base, From: local, To: remote})
	}
	if b, err := os.ReadFile(dockerFile); err == nil {
		var st map[string]struct {
			Status string `json:"status"`
		}
		if json.Unmarshal(b, &st) == nil {
			names := make([]string, 0, len(st))
			for n := range st {
				names = append(names, n)
			}
			sort.Strings(names)
			for _, n := range names {
				// "true": up to date, "false": a newer image exists.
				if st[n].Status == "false" {
					out = append(out, Update{Kind: "container", Name: strings.TrimPrefix(n, "library/"), To: "new image"})
				}
			}
		}
	}
	return out
}

// ---- Network rates ----

type netCounters struct{ rx, tx uint64 }

func readNetCounters(path string) map[string]netCounters {
	out := map[string]netCounters{}
	b, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(b), "\n") {
		name, rest, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		f := strings.Fields(rest)
		if len(f) < 9 {
			continue
		}
		rx, _ := strconv.ParseUint(f[0], 10, 64)
		tx, _ := strconv.ParseUint(f[8], 10, 64)
		out[strings.TrimSpace(name)] = netCounters{rx, tx}
	}
	return out
}

// ---- Alarms ----

func hottestBay(bays []Bay) (Bay, bool) {
	var best Bay
	found := false
	for _, b := range bays {
		if b.Present && b.HDD && !b.Standby && b.Temp != nil && (!found || *b.Temp > *best.Temp) {
			best, found = b, true
		}
	}
	return best, found
}

// buildAlarms says what is wrong: what the fan daemon's fail-safe reports, plus the limits checked here too,
// so the display shows an alarm even if the daemon does not run.
func buildAlarms(cpu *int, bays []Bay, nvme []NVMe, st *State, cfg Config) []Alarm {
	out := []Alarm{}
	seen := map[string]bool{}
	add := func(a Alarm) {
		if !seen[a.Text] {
			seen[a.Text] = true
			out = append(out, a)
		}
	}
	hint := ""
	if st != nil && st.Failsafe != "" {
		hint = "fans at full speed"
	}
	if cpu != nil && *cpu >= cfg.CPUCrit {
		add(Alarm{Level: "crit", Code: "cpu_hot", Text: "CPU too hot", Value: cpu, Limit: cfg.CPUCrit, Hint: hint})
	}
	if b, ok := hottestBay(bays); ok && *b.Temp >= cfg.DiskCrit {
		add(Alarm{Level: "crit", Code: "disk_hot", Bay: b.Bay, Text: fmt.Sprintf("Drive %d too hot", b.Bay), Value: b.Temp, Limit: cfg.DiskCrit, Hint: hint})
	}
	for _, n := range nvme {
		if n.Temp != nil && *n.Temp >= cfg.DiskCrit+cfg.NVMeOffset {
			add(Alarm{Level: "crit", Code: "nvme_hot", Text: "NVMe too hot", Value: n.Temp, Limit: cfg.DiskCrit + cfg.NVMeOffset, Hint: hint})
		}
	}
	if st != nil {
		switch st.Failsafe {
		case "cpu_unreadable":
			add(Alarm{Level: "crit", Code: "cpu_unreadable", Text: "CPU temperature unreadable", Hint: hint})
		case "cpu_hot":
			add(Alarm{Level: "crit", Code: "cpu_hot", Text: "CPU too hot", Value: st.CPUTemp, Limit: cfg.CPUCrit, Hint: hint})
		case "disk_hot":
			add(Alarm{Level: "crit", Code: "disk_hot", Text: "Drive too hot", Limit: cfg.DiskCrit, Hint: hint})
		case "nvme_hot":
			add(Alarm{Level: "crit", Code: "nvme_hot", Text: "NVMe too hot", Hint: hint})
		}
		for _, f := range st.Stalled {
			add(Alarm{Level: "warning", Code: "fan_stalled", Text: "Fan stalled: " + f})
		}
	}
	return out
}

// ---- The state ----

type collector struct {
	cfgPath  string
	hw       Hardware
	hwAt     time.Time
	prevNet  map[string]netCounters
	prevAt   time.Time
	rates    map[string][2]int64
	statusFn func() (Sensors, Info, []FanStatus, bool)
}

func collectLive() (Sensors, Info, []FanStatus, bool) {
	sensors := ReadSensors()
	info := ReadInfo()
	fanOn := LoadConfig(configPath).FanControl
	var fans []FanStatus
	if fanOn {
		out := map[string]any{}
		addChipStatus(out, &sensors)
		fans, _ = out["fans"].([]FanStatus)
	}
	return sensors, info, fans, fanOn
}

func (c *collector) netRates(now time.Time) {
	cur := readNetCounters(procNetDev)
	if c.prevNet != nil && now.After(c.prevAt) {
		dt := now.Sub(c.prevAt).Seconds()
		c.rates = map[string][2]int64{}
		for n, v := range cur {
			if p, ok := c.prevNet[n]; ok && v.rx >= p.rx && v.tx >= p.tx {
				c.rates[n] = [2]int64{int64(float64(v.rx-p.rx) / dt), int64(float64(v.tx-p.tx) / dt)}
			}
		}
	}
	c.prevNet, c.prevAt = cur, now
}

func (c *collector) state(now time.Time) PubState {
	sensors, info, fans, fanOn := c.statusFn()
	cfg := LoadConfig(c.cfgPath)

	if now.Sub(c.hwAt) > time.Minute {
		c.hw, c.hwAt = ReadHardware(), now
	}
	c.netRates(now)

	var st *State
	if b, err := os.ReadFile(statePath); err == nil {
		var s State
		if json.Unmarshal(b, &s) == nil && now.Unix()-s.Time < 30 {
			st = &s
		}
	}

	s := PubState{
		Time: now.Unix(), Model: c.hw.Model, Unraid: info.Unraid, Uptime: info.Uptime, Mode: cfg.Mode, FanControl: fanOn,
		CPU: sensors.CPU, Board: sensors.Board, Fans: []PubFan{}, Bays: c.hw.Bays, NVMe: []PubNVMe{}, Net: []PubNet{},
	}
	if s.Bays == nil {
		s.Bays = []Bay{}
	}
	s.Storage, s.Parity = readStorage(disksINI), readParity(varINI)
	if st != nil {
		s.CaseTemp = st.CaseTemp
		s.Mode = st.Mode
	}
	for _, f := range fans {
		s.Fans = append(s.Fans, PubFan{ID: f.ID, Role: f.Role, RPM: f.RPM, Pct: f.Pct, Auto: f.Auto, Error: f.Err})
	}
	for i, n := range c.hw.NVMe {
		name := n.Role
		if name == "" {
			name = fmt.Sprintf("NVMe %d", i+1)
		}
		s.NVMe = append(s.NVMe, PubNVMe{Name: name, Slot: n.Slot, Temp: n.Temp})
	}
	for _, p := range c.hw.Net {
		r := c.rates[p.Name]
		s.Net = append(s.Net, PubNet{Name: p.Name, Link: p.Link, SpeedMbit: p.SpeedMbit, MaxMbit: p.MaxMbit, RxBps: r[0], TxBps: r[1]})
	}
	s.Alarms = buildAlarms(sensors.CPU, c.hw.Bays, c.hw.NVMe, st, cfg)
	return s
}

// ---- Commands ----

type command struct {
	Cmd    string `json:"cmd"`
	Target string `json:"target"`
	Name   string `json:"name"`
	TS     int64  `json:"ts"`
	Nonce  string `json:"nonce"`
}

type cmdResult struct {
	Nonce string `json:"nonce"`
	OK    bool   `json:"ok"`
	Text  string `json:"text"`
}

const (
	cmdMaxAge   = 30 * time.Second
	cmdMinSpace = 10 * time.Second
)

// commander validates and carries out commands. run is how a program is started (replaced in tests).
type commander struct {
	cfg     MQTTConfig
	updates func() []Update
	// containers returns the names of the containers that were created from an image.
	containers func(image string) ([]string, error)
	run        func(name string, args ...string) error
	delay      time.Duration

	mu    sync.Mutex
	seen  map[string]time.Time
	lastC time.Time
}

// handle checks one command and returns the answer. The program is started by `after`, which the caller runs once
// the answer has been sent: a restart would otherwise take the answer down with it.
func (c *commander) handle(raw []byte, now time.Time) (res cmdResult, after func()) {
	var cmd command
	if err := json.Unmarshal(raw, &cmd); err != nil {
		return cmdResult{Text: "unreadable command"}, nil
	}
	res.Nonce = cmd.Nonce
	reject := func(text string) (cmdResult, func()) {
		fmt.Printf("command %q rejected: %s\n", cmd.Cmd, text)
		res.Text = text
		return res, nil
	}

	if cmd.Nonce == "" || len(cmd.Nonce) > 64 {
		return reject("nonce missing")
	}
	age := now.Sub(time.Unix(cmd.TS, 0))
	if age > cmdMaxAge || age < -cmdMaxAge {
		return reject("command too old or clock apart")
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.seen == nil {
		c.seen = map[string]time.Time{}
	}
	for n, t := range c.seen {
		if now.Sub(t) > 2*cmdMaxAge {
			delete(c.seen, n)
		}
	}
	if _, dup := c.seen[cmd.Nonce]; dup {
		return reject("command already seen")
	}
	if len(c.seen) >= 256 {
		return reject("too many commands")
	}
	c.seen[cmd.Nonce] = now
	if now.Sub(c.lastC) < cmdMinSpace {
		return reject("too fast, wait a moment")
	}

	var progs [][]string
	switch cmd.Cmd {
	case "reboot", "shutdown":
		if !c.cfg.Control {
			return reject("remote control is switched off")
		}
		prog := []string{powerdownBin}
		if cmd.Cmd == "reboot" {
			prog = append(prog, "-r")
		}
		progs = [][]string{prog}
	case "update":
		switch cmd.Target {
		case "plugin":
			if !c.cfg.ControlUpdates {
				return reject("updates from the display are switched off")
			}
			file := ""
			for _, u := range c.updates() {
				if u.Kind == "plugin" && u.Name == cmd.Name {
					file = u.File
				}
			}
			if file == "" || filepath.Base(file) != file {
				return reject("no such update")
			}
			progs = [][]string{{pluginBin, "update", file}}
		case "container":
			if !c.cfg.ControlContainers {
				return reject("container updates from the display are switched off")
			}
			known := false
			for _, u := range c.updates() {
				if u.Kind == "container" && u.Name == cmd.Name {
					known = true
				}
			}
			if !known {
				return reject("no such update")
			}
			names, err := c.containers(cmd.Name)
			if err != nil {
				return reject("docker could not be run: " + err.Error())
			}
			if len(names) == 0 {
				return reject("no container found for that image")
			}
			for _, n := range names {
				if !containerNameRe.MatchString(n) {
					return reject("container name not allowed")
				}
				progs = append(progs, []string{updateContainerBin, n})
			}
		default:
			return reject("only plugin and container updates can be installed this way")
		}
	default:
		return reject("unknown command")
	}

	c.lastC = now
	for _, p := range progs {
		fmt.Printf("command %q accepted: %s\n", cmd.Cmd, strings.Join(p, " "))
	}
	res.OK, res.Text = true, "started"
	return res, func() {
		time.Sleep(c.delay)
		for _, p := range progs {
			if err := c.run(p[0], p[1:]...); err != nil {
				fmt.Printf("command %q failed: %v\n", cmd.Cmd, err)
			}
		}
	}
}

// containerNameRe is what Docker allows for a container name; anything else is not passed to a program.
var containerNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)

// dockerContainers lists the containers created from image. Unraid names an update by its image, but the
// update script wants the container's name.
func dockerContainers(image string) ([]string, error) {
	out, err := exec.Command(dockerBin, "ps", "-a", "--format", "{{.Names}}\t{{.Image}}").Output()
	if err != nil {
		return nil, err
	}
	return matchContainers(string(out), image), nil
}

func matchContainers(listing, image string) []string {
	var names []string
	for _, l := range strings.Split(listing, "\n") {
		f := strings.SplitN(l, "\t", 2)
		if len(f) == 2 && normImage(f[1]) == normImage(image) {
			names = append(names, strings.TrimSpace(f[0]))
		}
	}
	return names
}

// normImage makes the two spellings of one image equal: Unraid lists "wordpress:latest" where docker ps shows
// the image as the container was created, "wordpress"; "library/" and "docker.io/" are the same registry paths.
func normImage(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "docker.io/")
	s = strings.TrimPrefix(s, "library/")
	if i := strings.LastIndex(s, "/"); !strings.Contains(s[i+1:], ":") && !strings.Contains(s[i+1:], "@") {
		s += ":latest"
	}
	return s
}

func execRun(name string, args ...string) error {
	return exec.Command(name, args...).Run()
}

// ---- The loop ----

func runPublish() error {
	cfg := LoadMQTTConfig(configPath)
	if !cfg.Enabled {
		return fmt.Errorf("MQTT is switched off in the settings")
	}
	if cfg.Host == "" {
		return fmt.Errorf("no MQTT host in the settings")
	}

	prefix := "ugreen-nas/" + cfg.Name + "/"
	col := &collector{cfgPath: configPath, statusFn: collectLive}
	cmdr := &commander{cfg: cfg, run: execRun, containers: dockerContainers, delay: time.Second,
		updates: func() []Update { return readUpdates(pluginsBoot, pluginsFresh, dockerStatus, unraidCheck) }}

	client := &mqttClient{
		addr: fmt.Sprintf("%s:%d", cfg.Host, cfg.Port), useTLS: cfg.TLS, user: cfg.User, pass: cfg.Pass,
		clientID:  "ugreen-nas-" + cfg.Name,
		willTopic: prefix + "availability", willPayload: []byte("offline"),
	}
	last := map[string][]byte{}
	send := func(topic string, v any, retain bool) {
		b, err := json.Marshal(v)
		if err != nil {
			return
		}
		if retain && string(last[topic]) == string(b) {
			return
		}
		if client.publish(prefix+topic, b, retain) == nil {
			last[topic] = b
		}
	}
	var lastMu sync.Mutex
	client.onConnect = func() {
		lastMu.Lock()
		last = map[string][]byte{}
		lastMu.Unlock()
		_ = client.publish(prefix+"availability", []byte("online"), true)
		if cfg.Control || cfg.ControlUpdates || cfg.ControlContainers {
			_ = client.subscribe(prefix + "cmd")
		}
	}
	client.onMessage = func(topic string, payload []byte) {
		if topic != prefix+"cmd" {
			return
		}
		res, after := cmdr.handle(payload, time.Now())
		b, _ := json.Marshal(res)
		_ = client.publish(prefix+"cmd_result", b, false)
		if after != nil {
			go after()
		}
	}

	stop := make(chan struct{})
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	go func() { <-sig; close(stop) }()
	go client.run(stop)

	tick := time.NewTicker(cfg.Interval)
	defer tick.Stop()
	var slowAt time.Time
	for {
		now := time.Now()
		lastMu.Lock()
		send("state", col.state(now), true)
		if now.Sub(slowAt) >= time.Minute || slowAt.IsZero() {
			slowAt = now
			send("updates", readUpdates(pluginsBoot, pluginsFresh, dockerStatus, unraidCheck), true)
		}
		send("notifications", readNotifications(notifyDir, now), true)
		lastMu.Unlock()

		select {
		case <-tick.C:
		case <-stop:
			_ = client.publish(prefix+"availability", []byte("offline"), true)
			time.Sleep(200 * time.Millisecond)
			return nil
		}
	}
}
