package nasctl

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLEDEnableInitializesOnlyFreshSettings(t *testing.T) {
	for _, tc := range []struct {
		name, saved, imported string
		exists                bool
	}{
		{name: "absent"},
		{name: "empty", exists: true},
		{name: "whitespace", saved: " \n", exists: true},
		{name: "manual off at night", saved: "mode=\"manual\"\npower_brightness=\"0\"\nnight_enabled=\"1\"\n", exists: true},
		{name: "imported manual", imported: "mode=\"manual\"\npower_brightness=\"40\"\n"},
		{name: "saved beats import", exists: true, saved: "mode=\"manual\"\npower_brightness=\"0\"\n", imported: "mode=\"status\"\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			t.Setenv("PATH", f.dir)
			write(t, filepath.Join(f.c.Env.DMIDir, "product_name"), "iDX6011\n")
			if tc.exists {
				write(t, f.c.Env.LEDCfg, tc.saved)
			}
			if tc.imported != "" {
				if err := os.MkdirAll(filepath.Dir(f.c.Env.LEDImportSrc), 0o755); err != nil {
					t.Fatal(err)
				}
				write(t, f.c.Env.LEDImportSrc, tc.imported)
			}
			calls := filepath.Join(f.dir, "calls")
			for path, script := range map[string]string{
				f.c.Env.LEDI2C:     "#!/bin/sh\necho '{\"bus\":\"/dev/i2c-0\"}'\n",
				f.c.Env.LEDCtl:     "#!/bin/sh\nprintf '%s\\n' \"$*\" >> '" + calls + "'\n",
				f.c.Env.UpdateCron: "#!/bin/sh\nexit 0\n",
			} {
				write(t, path, script)
				if err := os.Chmod(path, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if rc := f.c.ledEnable(); rc != 0 {
				t.Fatalf("enable: %d %s", rc, f.errw)
			}
			data, err := os.ReadFile(f.c.Env.LEDCfg)
			if err != nil {
				t.Fatal(err)
			}
			want := tc.saved
			if !tc.exists {
				want = tc.imported
			}
			fresh := strings.TrimSpace(want) == ""
			if fresh {
				for _, setting := range []string{"mode=\"status\"", "power_effect=\"none\"", "power_brightness=\"255\""} {
					if !strings.Contains(string(data), setting) {
						t.Fatalf("missing %s in %q", setting, data)
					}
				}
			} else if string(data) != want {
				t.Fatalf("settings changed: %q, want %q", data, want)
			}
			log, _ := os.ReadFile(calls)
			if !strings.Contains(string(log), "apply\n") || strings.Contains(string(log), "daemon restart\n") != fresh {
				t.Fatalf("boot calls %q", log)
			}
			if rc := f.c.ledEnable(); rc != 0 {
				t.Fatal("second enable failed")
			}
			again, _ := os.ReadFile(f.c.Env.LEDCfg)
			if string(data) != string(again) {
				t.Fatal("second enable changed settings")
			}
		})
	}
}

func TestLEDEnableSettingsFailureDoesNotEnable(t *testing.T) {
	f := newFixture(t)
	if err := os.Mkdir(f.c.Env.LEDCfg, 0o755); err != nil {
		t.Fatal(err)
	}
	if rc := f.c.ledEnable(); rc == 0 {
		t.Fatal("initialization failure reported success")
	}
	if _, err := os.Stat(f.c.Env.LEDFlag); !os.IsNotExist(err) {
		t.Fatal("enabled LEDs despite invalid settings")
	}
}
