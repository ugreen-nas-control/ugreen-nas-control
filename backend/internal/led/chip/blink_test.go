package chip

import (
	"bytes"
	"errors"
	"testing"
)

type blinkResetBus struct {
	brightness map[byte]int
	mode       map[byte]string
	timing     map[byte][2]int
	reads      int
	writes     []byte
	failRead   bool
	failWrite  byte
	dropWrites int
}

func (b *blinkResetBus) close()          {}
func (b *blinkResetBus) busPath() string { return "/dev/fake" }
func (b *blinkResetBus) status(name string) (ledStatus, error) {
	b.reads++
	if b.failRead {
		return ledStatus{}, errors.New("read failed")
	}
	id, _ := ledID(name)
	return ledStatus{Name: name, Brightness: b.brightness[id], Mode: b.mode[id], OnMs: b.timing[id][0], OffMs: b.timing[id][1]}, nil
}
func (b *blinkResetBus) change(id, command byte, params [4]byte) error {
	b.writes = append(b.writes, command)
	if command == b.failWrite {
		return errors.New("write failed")
	}
	if b.dropWrites > 0 {
		b.dropWrites--
		return nil
	}
	if b.mode == nil {
		b.mode = map[byte]string{}
	}
	if b.timing == nil {
		b.timing = map[byte][2]int{}
	}
	switch command {
	case cmdBlink:
		b.mode[id] = "blink"
		b.brightness[id] = 180
		on := int(params[2])<<8 | int(params[3])
		cycle := int(params[0])<<8 | int(params[1])
		b.timing[id] = [2]int{on, cycle - on}
	case cmdBrightness:
		b.brightness[id] = int(params[0])
	case cmdOnOff:
		b.mode[id] = modeNames[params[0]]
	}
	return nil
}

func TestGTBlinkPreservesEachBrightness(t *testing.T) {
	oldBus, oldLayout := openLedBus, getLayout()
	t.Cleanup(func() { openLedBus = oldBus; SetLayout(oldLayout) })
	SetLayout("gt")
	b := &blinkResetBus{brightness: map[byte]int{2: 32, 3: 255}}
	openLedBus = func(string, bool) (ledBus, error) { return b, nil }
	var out, errw bytes.Buffer
	if rc := Run([]string{"set", "disk1,disk2", "blink", "100", "100"}, &out, &errw); rc != 0 {
		t.Fatalf("rc %d: %s", rc, errw.String())
	}
	if b.brightness[2] != 32 || b.brightness[3] != 255 {
		t.Fatalf("brightness after blink = %v", b.brightness)
	}
}

func TestGTBlinkStopsOnReadOrRestoreFailure(t *testing.T) {
	oldBus, oldLayout := openLedBus, getLayout()
	t.Cleanup(func() { openLedBus = oldBus; SetLayout(oldLayout) })
	SetLayout("gt")
	for _, failRead := range []bool{true, false} {
		b := &blinkResetBus{brightness: map[byte]int{2: 32}, failRead: failRead, failWrite: cmdBrightness}
		openLedBus = func(string, bool) (ledBus, error) { return b, nil }
		var out, errw bytes.Buffer
		if rc := Run([]string{"set", "disk1", "blink", "100", "100"}, &out, &errw); rc == 0 {
			t.Fatal("reported success after a failed read or restore")
		}
		if failRead && len(b.writes) != 0 {
			t.Fatal("changed blink without a brightness snapshot")
		}
	}
}

func TestOtherLayoutsDoNotUseGTBlinkWorkaround(t *testing.T) {
	oldBus, oldLayout := openLedBus, getLayout()
	t.Cleanup(func() { openLedBus = oldBus; SetLayout(oldLayout) })
	SetLayout("idx-pro")
	b := &blinkResetBus{brightness: map[byte]int{3: 32}}
	openLedBus = func(string, bool) (ledBus, error) { return b, nil }
	var out, errw bytes.Buffer
	if rc := Run([]string{"set", "disk1", "blink", "100", "100"}, &out, &errw); rc != 0 || b.reads != 0 || len(b.writes) != 1 {
		t.Fatalf("rc %d reads %d writes %v: %s", rc, b.reads, b.writes, errw.String())
	}
}

func TestGTBrightnessRetriesAcknowledgedButUnappliedWrite(t *testing.T) {
	oldBus, oldLayout := openLedBus, getLayout()
	t.Cleanup(func() { openLedBus = oldBus; SetLayout(oldLayout) })
	SetLayout("gt")
	b := &blinkResetBus{brightness: map[byte]int{2: 180}, dropWrites: 1}
	openLedBus = func(string, bool) (ledBus, error) { return b, nil }
	var out, errw bytes.Buffer
	if rc := Run([]string{"set", "disk1", "brightness", "32"}, &out, &errw); rc != 0 || b.brightness[2] != 32 || len(b.writes) != 2 {
		t.Fatalf("rc %d, brightness %v, writes %v: %s", rc, b.brightness, b.writes, errw.String())
	}
	b.dropWrites = retries
	if rc := Run([]string{"set", "disk1", "brightness", "99"}, &out, &errw); rc == 0 {
		t.Fatal("reported success although all acknowledged writes were ignored")
	}
}

func TestGTRetriesAcknowledgedButUnappliedModeChange(t *testing.T) {
	oldBus, oldLayout := openLedBus, getLayout()
	t.Cleanup(func() { openLedBus = oldBus; SetLayout(oldLayout) })
	SetLayout("gt")
	b := &blinkResetBus{brightness: map[byte]int{2: 32}, mode: map[byte]string{2: "blink"}, dropWrites: 1}
	openLedBus = func(string, bool) (ledBus, error) { return b, nil }
	var out, errw bytes.Buffer
	if rc := Run([]string{"set", "disk1", "off"}, &out, &errw); rc != 0 || b.mode[2] != "off" || len(b.writes) != 2 {
		t.Fatalf("rc %d, mode %v, writes %v: %s", rc, b.mode, b.writes, errw.String())
	}
}

func TestGTRetriesUnappliedBlinkTimingEvenWhenModeMatches(t *testing.T) {
	oldBus, oldLayout := openLedBus, getLayout()
	t.Cleanup(func() { openLedBus = oldBus; SetLayout(oldLayout) })
	SetLayout("gt")
	b := &blinkResetBus{brightness: map[byte]int{2: 32}, mode: map[byte]string{2: "blink"}, timing: map[byte][2]int{2: {200, 300}}, dropWrites: 1}
	openLedBus = func(string, bool) (ledBus, error) { return b, nil }
	var out, errw bytes.Buffer
	if rc := Run([]string{"set", "disk1", "blink", "100", "100"}, &out, &errw); rc != 0 || b.timing[2] != [2]int{100, 100} {
		t.Fatalf("rc %d, timing %v: %s", rc, b.timing, errw.String())
	}
}
