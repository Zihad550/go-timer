package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"
)

type TimerSummary struct {
	Start    time.Time
	End      time.Time
	Duration time.Duration
	Mode     string // "timer" or "counter"
	Finished bool   // true if completed, false if quit/interrupted
	Name     string // optional name for the timer
}

type Session struct {
	Start     string `json:"start"`
	Current   string `json:"current"`
	Elapsed   string `json:"elapsed"`
	Remaining string `json:"remaining,omitempty"` // Only for timer mode
	Paused    bool   `json:"paused"`
	Mode      string `json:"mode"` // "timer" or "counter"
	Name      string `json:"name,omitempty"`
	Finished  bool   `json:"finished"`
	Inline    bool   `json:"inline"` // true if inline mode, false if fullscreen
}

func addSuffixIfArgIsNumber(s *string, suffix string) {
	_, err := strconv.ParseFloat(*s, 64)
	if err == nil {
		*s = *s + suffix
	}
}

func formatDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

func parseFormattedDuration(s string) time.Duration {
	if s == "0s" {
		return 0
	}
	var sec float64
	_, err := fmt.Sscanf(s, "%fs", &sec)
	if err != nil {
		return 0
	}
	return time.Duration(sec * float64(time.Second))
}

func loadSession(name string) (Session, error) {
	if name == "" {
		name = "default"
	}
	data, err := os.ReadFile("sessions.json")
	if err != nil {
		return Session{}, fmt.Errorf("failed to read sessions.json: %w", err)
	}
	var sessions map[string]Session
	if err := json.Unmarshal(data, &sessions); err != nil {
		return Session{}, fmt.Errorf("failed to parse sessions.json: %w", err)
	}
	session, ok := sessions[name]
	if !ok {
		return Session{}, fmt.Errorf("session %q not found", name)
	}
	return session, nil
}
