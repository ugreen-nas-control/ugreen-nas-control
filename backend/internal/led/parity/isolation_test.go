package parity

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ugreen-nas-control/ugreen-nas-control/internal/led/ledctl"
)

func TestFixtureSystemPathsAreIsolated(t *testing.T) {
	r := newRig(t, "go", "DXP6800 Pro")
	env := reflect.ValueOf(ledctl.EnvFromGetenv(func(key string) string { return r.env[key] }, r.ctl))
	for i := range env.NumField() {
		field := env.Type().Field(i)
		if field.Type.Kind() != reflect.String {
			continue
		}
		switch field.Name {
		case "Product", "Now", "Tool":
			continue
		case "Proc":
			// Process ownership checks must see the real child processes the fixture starts.
			if env.Field(i).String() != "/proc" {
				t.Fatal("process ownership no longer uses the live process table")
			}
			continue
		}
		rel, err := filepath.Rel(r.root, env.Field(i).String())
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			t.Errorf("%s escapes the test fixture: %q", field.Name, env.Field(i).String())
		}
	}
}
