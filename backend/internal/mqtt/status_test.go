package mqtt

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeBroker answers the first connection's CONNECT with a CONNACK of the given code and returns what the client sent.
func fakeBroker(t *testing.T, code byte) (addr string, got chan []byte) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	got = make(chan []byte, 2)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer func() { _ = conn.Close() }()
		r := bufio.NewReader(conn)
		typ, _, body, _ := readPacket(r)
		if typ == mqttConnect {
			got <- body
		}
		_, _ = conn.Write([]byte{mqttConnack << 4, 2, 0, code})
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		if typ, _, _, err := readPacket(r); err == nil && typ == mqttDisconn {
			got <- []byte("disconnect")
		}
	}()
	return ln.Addr().String(), got
}

func hostPort(t *testing.T, addr string) (string, int) {
	t.Helper()
	host, port, _ := net.SplitHostPort(addr)
	var n int
	if _, err := fmt.Sscan(port, &n); err != nil {
		t.Fatal(err)
	}
	return host, n
}

func TestStatusFileRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "status")
	if _, ok := ReadStatus(path); ok {
		t.Fatal("a missing file is no status")
	}
	writeStatus(path, Status{State: "failed", Reason: "dial tcp x: connect: no route to host"})
	s, ok := ReadStatus(path)
	if !ok || s.State != "failed" || s.Reason != "dial tcp x: connect: no route to host" || s.Time == 0 {
		t.Fatalf("read back %+v %v", s, ok)
	}
	for _, bad := range []string{"", "not json", `{"state":"weird"}`} {
		_ = os.WriteFile(path, []byte(bad), 0o644)
		if _, ok := ReadStatus(path); ok {
			t.Errorf("%q must not count as a status", bad)
		}
	}
}

func TestSessionReportsConnectedAndFailed(t *testing.T) {
	addr, _ := fakeBroker(t, 0)
	states := make(chan string, 4)
	c := &client{addr: addr, clientID: "id", onStatus: func(state, reason string) { states <- state + ":" + reason }}
	quit := make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- c.session(quit) }()
	select {
	case got := <-states:
		if got != "connected:" {
			t.Fatalf("state %q", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no state")
	}
	close(quit)
	<-done

	addr, _ = fakeBroker(t, 4)
	c = &client{addr: addr, clientID: "id"}
	err := c.session(make(chan struct{}))
	if err == nil || reasonOf(err) != "connection refused: bad user name or password (code 4)" {
		t.Fatalf("reason %v", err)
	}
}

func TestTestUsesItsOwnClientIDAndHangsUp(t *testing.T) {
	addr, got := fakeBroker(t, 0)
	host, port := hostPort(t, addr)
	if err := Test(Config{Host: host, Port: port, User: "nas", Pass: "secret"}); err != nil {
		t.Fatal(err)
	}
	connect := string(<-got)
	if !strings.Contains(connect, "ugreen-nas-test-") || strings.Contains(connect, "ugreen-nas-nas") {
		t.Errorf("client identifier: %q", connect)
	}
	if !strings.Contains(connect, "secret") {
		t.Error("the password must be sent")
	}
	select {
	case m := <-got:
		if string(m) != "disconnect" {
			t.Errorf("got %q", m)
		}
	case <-time.After(2 * time.Second):
		t.Error("no DISCONNECT")
	}
}

func TestRunTestAnswers(t *testing.T) {
	answer := func(in string) (map[string]any, error) {
		var out bytes.Buffer
		err := RunTest(strings.NewReader(in), &out)
		var m map[string]any
		if err == nil {
			if jerr := json.Unmarshal(out.Bytes(), &m); jerr != nil {
				t.Fatalf("answer %q: %v", out.String(), jerr)
			}
		}
		return m, err
	}

	addr, _ := fakeBroker(t, 0)
	host, port := hostPort(t, addr)
	m, err := answer(fmt.Sprintf(`{"host":%q,"port":%d}`, host, port))
	if err != nil || m["ok"] != true || m["connected"] != true {
		t.Errorf("connect: %v %v", m, err)
	}

	addr, _ = fakeBroker(t, 5)
	host, port = hostPort(t, addr)
	m, err = answer(fmt.Sprintf(`{"host":%q,"port":%d,"user":"a","pass":"b"}`, host, port))
	if err != nil || m["connected"] != false || m["reason"] != "connection refused: not authorized (code 5)" {
		t.Errorf("refused: %v %v", m, err)
	}

	// nothing listens on the port the listener just gave up
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	host, port = hostPort(t, ln.Addr().String())
	_ = ln.Close()
	m, err = answer(fmt.Sprintf(`{"host":%q,"port":%d}`, host, port))
	if err != nil || m["connected"] != false || !strings.Contains(fmt.Sprint(m["reason"]), "connection refused") {
		t.Errorf("unreachable: %v %v", m, err)
	}

	for _, bad := range []string{``, `not json`, `{}`, `{"host":""}`} {
		if _, err := answer(bad); err == nil {
			t.Errorf("%q must be an error", bad)
		}
	}
}
