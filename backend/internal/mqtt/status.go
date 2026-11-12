package mqtt

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// Status is the connection of the running publisher, kept in StatusPath for the settings page and diag. The file exists
// while the publisher runs; it holds the last change: connecting (before the first CONNACK), connected, or failed with
// the reason (the log line without its prefix) while the publisher retries.
type Status struct {
	State  string `json:"state"`
	Reason string `json:"reason,omitempty"`
	Time   int64  `json:"time"`
}

// StatusPath is where Run keeps its Status; ugreen-nas-ctl and diag take it as their default and remove it when the publisher stops.
var StatusPath = envOr("UGREEN_PUB_STATUS", "/var/run/ugreen-nas-control-mqtt.status")

func writeStatus(path string, s Status) {
	s.Time = time.Now().Unix()
	b, err := json.Marshal(s)
	if err != nil {
		return
	}
	tmp := path + ".tmp"
	if os.WriteFile(tmp, b, 0o644) != nil || os.Rename(tmp, path) != nil {
		_ = os.Remove(tmp)
	}
}

// ReadStatus reads a status file; false when it is missing or not a status.
func ReadStatus(path string) (Status, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Status{}, false
	}
	var s Status
	if json.Unmarshal(b, &s) != nil {
		return Status{}, false
	}
	switch s.State {
	case "connecting", "connected", "failed":
		return s, true
	}
	return Status{}, false
}

// reasonOf is an error as the settings page shows it: without the "mqtt: " prefix of the log.
func reasonOf(err error) string { return strings.TrimPrefix(err.Error(), "mqtt: ") }

// Test connects once the way the publisher does, with other settings, and hangs up. It uses its own client identifier,
// so a publisher that is running keeps its session (a second connection with the same one would replace it).
func Test(cfg Config) error {
	cl := &client{
		addr: fmt.Sprintf("%s:%d", cfg.Host, cfg.Port), useTLS: cfg.TLS, user: cfg.User, pass: cfg.Pass,
		clientID: fmt.Sprintf("ugreen-nas-test-%d", os.Getpid()),
	}
	conn, _, err := cl.dial()
	if err != nil {
		return err
	}
	_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	_, _ = conn.Write(packet(mqttDisconn, 0, nil))
	return conn.Close()
}

type testRequest struct {
	Host string `json:"host"`
	Port int    `json:"port"`
	TLS  bool   `json:"tls"`
	User string `json:"user"`
	Pass string `json:"pass"`
}

// RunTest is `ugreen-nas-fan mqtt-test`: the settings as JSON on in, the answer on out. A broker that cannot be reached
// or refuses the login is an answer ({"ok":true,"connected":false,"reason":...}); only unusable settings are an error.
func RunTest(in io.Reader, out io.Writer) error {
	var req testRequest
	if err := json.NewDecoder(io.LimitReader(in, 1<<16)).Decode(&req); err != nil {
		return fmt.Errorf("no settings to test")
	}
	if req.Host == "" {
		return errors.New("no MQTT host to test")
	}
	cfg := Config{Host: req.Host, Port: req.Port, TLS: req.TLS, User: req.User, Pass: req.Pass}
	if cfg.Port <= 0 || cfg.Port > 65535 {
		cfg.Port = 1883
		if cfg.TLS {
			cfg.Port = 8883
		}
	}
	answer := map[string]any{"ok": true, "connected": true}
	if err := Test(cfg); err != nil {
		answer["connected"] = false
		answer["reason"] = reasonOf(err)
	}
	return json.NewEncoder(out).Encode(answer)
}
