package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestAddSuffixIfArgIsNumber(t *testing.T) {
	t.Run("int", func(t *testing.T) {
		v := "5"
		addSuffixIfArgIsNumber(&v, "s")
		if v != "5s" {
			t.Fatalf("expected 5s, got %s", v)
		}
	})
	t.Run("float", func(t *testing.T) {
		v := "2.5"
		addSuffixIfArgIsNumber(&v, "s")
		if v != "2.5s" {
			t.Fatalf("expected 2.5s, got %s", v)
		}
	})
	t.Run("non-numeric", func(t *testing.T) {
		v := "5m"
		addSuffixIfArgIsNumber(&v, "s")
		if v != "5m" {
			t.Fatalf("expected 5m unchanged, got %s", v)
		}
	})
	t.Run("empty", func(t *testing.T) {
		v := ""
		addSuffixIfArgIsNumber(&v, "s")
		if v != "" {
			t.Fatalf("expected empty unchanged, got %s", v)
		}
	})
}

func TestFormatDuration(t *testing.T) {
	if got := formatDuration(0); got != "0s" {
		t.Fatalf("expected 0s, got %s", got)
	}
	if got := formatDuration(1500 * time.Millisecond); got != "1.5s" {
		t.Fatalf("expected 1.5s, got %s", got)
	}
	if got := formatDuration(10 * time.Second); got != "10.0s" {
		t.Fatalf("expected 10.0s, got %s", got)
	}
}

func TestParseFormattedDuration(t *testing.T) {
	if got := parseFormattedDuration("0s"); got != 0 {
		t.Fatalf("expected 0, got %v", got)
	}
	if got := parseFormattedDuration("1.5s"); got != 1500*time.Millisecond {
		t.Fatalf("expected 1.5s, got %v", got)
	}
	if got := parseFormattedDuration("10.0s"); got != 10*time.Second {
		t.Fatalf("expected 10s, got %v", got)
	}
	if got := parseFormattedDuration("bad"); got != 0 {
		t.Fatalf("expected 0 on bad input, got %v", got)
	}
}

func withTempDir(t *testing.T, fn func(dir string)) {
	t.Helper()
	dir, err := os.MkdirTemp("", "timer-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(dir)
	fn(dir)
}

func TestLoadSession(t *testing.T) {
	withTempDir(t, func(dir string) {
		cwd, err := os.Getwd()
		if err != nil {
			t.Fatalf("Getwd: %v", err)
		}
		defer os.Chdir(cwd)
		if err := os.Chdir(dir); err != nil {
			t.Fatalf("Chdir: %v", err)
		}

		sessions := map[string]Session{
			"default": {
				Start:    "2024-01-01:00-00-00",
				Current:  "2024-01-01:00-00-05",
				Elapsed:  "5.0s",
				Paused:   false,
				Mode:     "timer",
				Name:     "default",
				Finished: false,
				Inline:   false,
			},
			"named": {
				Start:    "2024-01-01:00-00-00",
				Current:  "2024-01-01:00-00-10",
				Elapsed:  "10.0s",
				Paused:   true,
				Mode:     "counter",
				Name:     "named",
				Finished: false,
				Inline:   true,
			},
		}
		data, err := json.Marshal(sessions)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		if err := os.WriteFile("sessions.json", data, 0644); err != nil {
			t.Fatalf("write sessions.json: %v", err)
		}

		if _, err := loadSession(""); err != nil {
			t.Fatalf("loadSession default: %v", err)
		}
		if _, err := loadSession("named"); err != nil {
			t.Fatalf("loadSession named: %v", err)
		}
		if _, err := loadSession("missing"); err == nil {
			t.Fatalf("expected error for missing session")
		}
	})
}

func TestGetTickerInterval(t *testing.T) {
	if got := getTickerInterval(0); got != tickIntervalFast {
		t.Fatalf("counter expected fast, got %v", got)
	}
	if got := getTickerInterval(30 * time.Second); got != tickIntervalFast {
		t.Fatalf("<1m expected fast, got %v", got)
	}
	if got := getTickerInterval(2 * time.Minute); got != tickIntervalMedium {
		t.Fatalf("2m expected medium, got %v", got)
	}
	if got := getTickerInterval(9*time.Minute + 59*time.Second); got != tickIntervalMedium {
		t.Fatalf("<10m expected medium, got %v", got)
	}
	if got := getTickerInterval(10 * time.Minute); got != tickIntervalSlow {
		t.Fatalf(">=10m expected slow, got %v", got)
	}
}

func TestParseInput(t *testing.T) {
	if b, ok := parseInput(nil); ok || b != 0 {
		t.Fatalf("empty: expected 0,false got %d,%v", b, ok)
	}
	if b, ok := parseInput([]byte{'a'}); !ok || b != 'a' {
		t.Fatalf("single: expected a,true got %d,%v", b, ok)
	}
	if b, ok := parseInput([]byte{0x1b}); ok || b != 0 {
		t.Fatalf("esc single: expected 0,false got %d,%v", b, ok)
	}
	if b, ok := parseInput([]byte{0x1b, '['}); ok || b != 0 {
		t.Fatalf("incomplete esc seq: expected 0,false got %d,%v", b, ok)
	}
	if b, ok := parseInput([]byte{0x1b, '[', 'M', 'x', 'y', 'M'}); !ok || b != 0 {
		t.Fatalf("mouse seq: expected 0,true got %d,%v", b, ok)
	}
}

func resetGlobals() {
	tickIntervalFast = 100 * time.Millisecond
	tickIntervalMedium = 500 * time.Millisecond
	tickIntervalSlow = 1 * time.Second
	warningThreshold = 5 * time.Minute
	glyphWidth = 8
	glyphHeight = 7
	glyphSpacing = 1
	keyBufferSize = 10
	defaultTermWidth = 80
	defaultTermHeight = 24
	restoreEnabled = false
}

func TestLoadConfigValid(t *testing.T) {
	defer resetGlobals()
	withTempDir(t, func(dir string) {
		configDir := filepath.Join(dir, "go-timer")
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		cfg := Config{
			TickIntervalFast:   200 * time.Millisecond,
			TickIntervalMedium: 600 * time.Millisecond,
			TickIntervalSlow:   1500 * time.Millisecond,
			WarningThreshold:   10 * time.Minute,
			GlyphWidth:         10,
			GlyphHeight:        9,
			GlyphSpacing:       2,
			KeyBufferSize:      20,
			DefaultTermWidth:   100,
			DefaultTermHeight:  40,
			Restore:            true,
		}
		b, err := json.Marshal(cfg)
		if err != nil {
			t.Fatalf("marshal cfg: %v", err)
		}
		if err := os.WriteFile(filepath.Join(configDir, "config.json"), b, 0644); err != nil {
			t.Fatalf("write config: %v", err)
		}

		origUserConfigDir := os.Getenv("XDG_CONFIG_HOME")
		defer os.Setenv("XDG_CONFIG_HOME", origUserConfigDir)
		if runtime.GOOS != "windows" {
			os.Setenv("XDG_CONFIG_HOME", dir)
		}

		loadConfig()

		if tickIntervalFast != cfg.TickIntervalFast {
			t.Fatalf("tickIntervalFast not updated")
		}
		if tickIntervalMedium != cfg.TickIntervalMedium {
			t.Fatalf("tickIntervalMedium not updated")
		}
		if tickIntervalSlow != cfg.TickIntervalSlow && cfg.TickIntervalSlow <= 5*time.Second {
			t.Fatalf("tickIntervalSlow not updated")
		}
		if warningThreshold != cfg.WarningThreshold {
			t.Fatalf("warningThreshold not updated")
		}
		if glyphWidth != cfg.GlyphWidth || glyphHeight != cfg.GlyphHeight || glyphSpacing != cfg.GlyphSpacing {
			t.Fatalf("glyph config not updated")
		}
		if keyBufferSize != cfg.KeyBufferSize {
			t.Fatalf("keyBufferSize not updated")
		}
		if defaultTermWidth != cfg.DefaultTermWidth || defaultTermHeight != cfg.DefaultTermHeight {
			t.Fatalf("term size not updated")
		}
		if !restoreEnabled {
			t.Fatalf("restoreEnabled not set")
		}
	})
}

func TestLoadConfigInvalid(t *testing.T) {
	defer resetGlobals()
	withTempDir(t, func(dir string) {
		configDir := filepath.Join(dir, "go-timer")
		if err := os.MkdirAll(configDir, 0755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		cfg := Config{
			TickIntervalFast:   1 * time.Millisecond,
			TickIntervalMedium: 2 * time.Second,
			TickIntervalSlow:   10 * time.Second,
			WarningThreshold:   10 * time.Second,
			GlyphWidth:         -1,
			GlyphHeight:        0,
			GlyphSpacing:       10,
			KeyBufferSize:      1000,
			DefaultTermWidth:   0,
			DefaultTermHeight:  0,
		}
		b, err := json.Marshal(cfg)
		if err != nil {
			t.Fatalf("marshal cfg: %v", err)
		}
		if err := os.WriteFile(filepath.Join(configDir, "config.json"), b, 0644); err != nil {
			t.Fatalf("write config: %v", err)
		}

		origUserConfigDir := os.Getenv("XDG_CONFIG_HOME")
		defer os.Setenv("XDG_CONFIG_HOME", origUserConfigDir)
		if runtime.GOOS != "windows" {
			os.Setenv("XDG_CONFIG_HOME", dir)
		}

		loadConfig()

		if tickIntervalFast != 100*time.Millisecond || tickIntervalMedium != 500*time.Millisecond || tickIntervalSlow != 1*time.Second {
			t.Fatalf("invalid durations should not override defaults")
		}
		if warningThreshold != 5*time.Minute {
			t.Fatalf("invalid warning threshold should not override default")
		}
		if glyphWidth != 8 || glyphHeight != 7 || glyphSpacing != 1 {
			t.Fatalf("invalid glyph config should not override defaults")
		}
		if keyBufferSize != 10 || defaultTermWidth != 80 || defaultTermHeight != 24 {
			t.Fatalf("invalid other config should not override defaults")
		}
	})
}

func TestWriteAndLoadSessionCompatibility(t *testing.T) {
	withTempDir(t, func(dir string) {
		cwd, err := os.Getwd()
		if err != nil {
			t.Fatalf("Getwd: %v", err)
		}
		defer os.Chdir(cwd)
		if err := os.Chdir(dir); err != nil {
			t.Fatalf("Chdir: %v", err)
		}

		s := Session{
			Start:    "2024-01-01:00-00-00",
			Current:  "2024-01-01:00-00-10",
			Elapsed:  "10.0s",
			Mode:     "timer",
			Name:     "",
			Finished: true,
			Inline:   true,
		}
		writeSession(s)
		loaded, err := loadSession("")
		if err != nil {
			t.Fatalf("loadSession after write: %v", err)
		}
		if loaded.Elapsed != s.Elapsed || loaded.Mode != s.Mode || loaded.Finished != s.Finished {
			t.Fatalf("loaded session does not match written")
		}
	})
}
