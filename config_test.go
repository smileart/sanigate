package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func Test_loadConfig_missing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	cfg, err := loadConfig(path)
	if !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected fs.ErrNotExist, got %v", err)
	}
	if cfg != (Config{}) {
		t.Errorf("expected zero Config on missing file, got %+v", cfg)
	}
}

func Test_loadConfig_roundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	body := `model = "gpt-4o"` + "\n" + `api_key = "sk-test"` + "\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}
	if cfg.Model != "gpt-4o" {
		t.Errorf("Model = %q, want gpt-4o", cfg.Model)
	}
	if cfg.APIKey != "sk-test" {
		t.Errorf("APIKey = %q, want sk-test", cfg.APIKey)
	}
}

func Test_loadConfig_rejectsLoosePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX-only check")
	}
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(`model = "gpt-4o"`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := loadConfig(path)
	if err == nil {
		t.Fatal("expected error on 0644 config, got nil")
	}
	if !strings.Contains(err.Error(), "chmod 0600") {
		t.Errorf("expected chmod hint in error, got: %v", err)
	}
}

func Test_loadConfig_parseError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("model = \"unterminated\nbroken"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := loadConfig(path)
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("expected path in error, got: %v", err)
	}
}

func Test_writeDefaultConfig_modes(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "sanigate")
	path := filepath.Join(dir, "config.toml")

	if err := writeDefaultConfig(path); err != nil {
		t.Fatalf("writeDefaultConfig failed: %v", err)
	}

	if runtime.GOOS != "windows" {
		dirInfo, err := os.Stat(dir)
		if err != nil {
			t.Fatal(err)
		}
		if perm := dirInfo.Mode().Perm(); perm != 0o700 {
			t.Errorf("dir perm = %#o, want 0o700", perm)
		}

		fileInfo, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if perm := fileInfo.Mode().Perm(); perm != 0o600 {
			t.Errorf("file perm = %#o, want 0o600", perm)
		}
	}

	// File must be loadable immediately after creation.
	cfg, err := loadConfig(path)
	if err != nil {
		t.Fatalf("loadConfig after writeDefaultConfig failed: %v", err)
	}
	if cfg.Model != "" || cfg.APIKey != "" {
		t.Errorf("expected zero Config from default body (commented out), got %+v", cfg)
	}
}

func Test_resolveModel(t *testing.T) {
	cases := []struct {
		name                  string
		flag, env, cfg, want  string
	}{
		{"flag wins over all", "flag-m", "env-m", "cfg-m", "flag-m"},
		{"env wins over cfg + default", "", "env-m", "cfg-m", "env-m"},
		{"cfg wins over default", "", "", "cfg-m", "cfg-m"},
		{"default applies when all empty", "", "", "", defaultModel},
		{"empty env treated as unset", "", "", "cfg-m", "cfg-m"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveModel(tc.flag, tc.env, tc.cfg)
			if got != tc.want {
				t.Errorf("resolveModel(%q, %q, %q) = %q, want %q",
					tc.flag, tc.env, tc.cfg, got, tc.want)
			}
		})
	}
}

func Test_resolveAPIKey(t *testing.T) {
	cases := []struct {
		name, env, cfg, want string
	}{
		{"env wins", "env-key", "cfg-key", "env-key"},
		{"cfg used when env empty", "", "cfg-key", "cfg-key"},
		{"both empty returns empty", "", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveAPIKey(tc.env, tc.cfg)
			if got != tc.want {
				t.Errorf("resolveAPIKey(%q, %q) = %q, want %q",
					tc.env, tc.cfg, got, tc.want)
			}
		})
	}
}
