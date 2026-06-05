package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const version = "dev"

// Configuration variables (defaults)
var (
	// Ticker intervals for different duration ranges
	tickIntervalFast   = 100 * time.Millisecond // For durations < 1 minute
	tickIntervalMedium = 500 * time.Millisecond // For durations 1-10 minutes
	tickIntervalSlow   = 1 * time.Second        // For durations > 10 minutes

	// Warning threshold for countdown timer
	warningThreshold = 5 * time.Minute

	// Big text glyph dimensions
	glyphWidth   = 8
	glyphHeight  = 7
	glyphSpacing = 1

	// Visual spacing (terminal line height cannot be changed, but we can adjust visual perception)

	// Keyboard input buffer size
	keyBufferSize = 10

	// Default terminal dimensions
	defaultTermWidth  = 80
	defaultTermHeight = 24

	// Auto-restore from last session
	restoreEnabled = true
)

// Config represents the configuration structure for config.json
type Config struct {
	TickIntervalFast   time.Duration `json:"tickIntervalFast"`
	TickIntervalMedium time.Duration `json:"tickIntervalMedium"`
	TickIntervalSlow   time.Duration `json:"tickIntervalSlow"`
	WarningThreshold   time.Duration `json:"warningThreshold"`
	GlyphWidth         int           `json:"glyphWidth"`
	GlyphHeight        int           `json:"glyphHeight"`
	GlyphSpacing       int           `json:"glyphSpacing"`
	KeyBufferSize      int           `json:"keyBufferSize"`
	DefaultTermWidth   int           `json:"defaultTermWidth"`
	DefaultTermHeight  int           `json:"defaultTermHeight"`
	Restore            *bool         `json:"restore"`
}

// loadConfig loads configuration from ~/.config/go-timer/config.json
// If the file doesn't exist or can't be read, it uses default values
func loadConfig() {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return // Use defaults
	}

	configPath := filepath.Join(configDir, "go-timer", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return // Use defaults
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return // Use defaults
	}

	// Apply config values with validation (non-zero for durations, positive for ints)
	if config.TickIntervalFast != 0 {
		if config.TickIntervalFast >= 10*time.Millisecond && config.TickIntervalFast <= 1*time.Second {
			tickIntervalFast = config.TickIntervalFast
		}
	}
	if config.TickIntervalMedium != 0 {
		if config.TickIntervalMedium >= 10*time.Millisecond && config.TickIntervalMedium <= 1*time.Second {
			tickIntervalMedium = config.TickIntervalMedium
		}
	}
	if config.TickIntervalSlow != 0 {
		if config.TickIntervalSlow >= 10*time.Millisecond && config.TickIntervalSlow <= 5*time.Second {
			tickIntervalSlow = config.TickIntervalSlow
		}
	}
	if config.WarningThreshold != 0 {
		if config.WarningThreshold >= 1*time.Minute && config.WarningThreshold <= 1*time.Hour {
			warningThreshold = config.WarningThreshold
		}
	}
	if config.GlyphWidth > 0 && config.GlyphWidth <= 20 {
		glyphWidth = config.GlyphWidth
	}
	if config.GlyphHeight > 0 && config.GlyphHeight <= 20 {
		glyphHeight = config.GlyphHeight
	}
	if config.GlyphSpacing >= 0 && config.GlyphSpacing <= 5 {
		glyphSpacing = config.GlyphSpacing
	}
	if config.KeyBufferSize > 0 && config.KeyBufferSize <= 100 {
		keyBufferSize = config.KeyBufferSize
	}
	if config.DefaultTermWidth > 0 && config.DefaultTermWidth <= 1000 {
		defaultTermWidth = config.DefaultTermWidth
	}
	if config.DefaultTermHeight > 0 && config.DefaultTermHeight <= 1000 {
		defaultTermHeight = config.DefaultTermHeight
	}
	if config.Restore != nil {
		restoreEnabled = *config.Restore
	}
}
