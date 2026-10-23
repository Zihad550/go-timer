package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
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

func getSessionsPath() string {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "sessions.json"
	}
	newPath := filepath.Join(configDir, "go-timer", "sessions.json")
	if _, err := os.Stat(newPath); err == nil {
		return newPath
	}
	oldPath := "sessions.json"
	if _, err := os.Stat(oldPath); err == nil {
		return oldPath
	}
	return newPath
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
	sessionsPath := getSessionsPath()
	data, err := os.ReadFile(sessionsPath)
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

func listSessions() (map[string]Session, error) {
	sessionsPath := getSessionsPath()
	data, err := os.ReadFile(sessionsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]Session), nil
		}
		return nil, fmt.Errorf("failed to read sessions.json: %w", err)
	}
	var sessions map[string]Session
	if err := json.Unmarshal(data, &sessions); err != nil {
		return nil, fmt.Errorf("failed to parse sessions.json: %w", err)
	}
	return sessions, nil
}

func saveSession(name string, session Session) error {
	if name == "" {
		name = "default"
	}
	sessionsPath := getSessionsPath()
	data, err := os.ReadFile(sessionsPath)
	sessions := make(map[string]Session)
	if err == nil {
		_ = json.Unmarshal(data, &sessions)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to read sessions.json: %w", err)
	}
	sessions[name] = session
	out, err := json.MarshalIndent(sessions, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(sessionsPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(sessionsPath, out, 0644)
}

func deleteSession(name string) error {
	if name == "" {
		name = "default"
	}
	sessionsPath := getSessionsPath()
	data, err := os.ReadFile(sessionsPath)
	if err != nil {
		return fmt.Errorf("failed to read sessions.json: %w", err)
	}
	var sessions map[string]Session
	if err := json.Unmarshal(data, &sessions); err != nil {
		return fmt.Errorf("failed to parse sessions.json: %w", err)
	}
	if _, ok := sessions[name]; !ok {
		return fmt.Errorf("session %q not found", name)
	}
	delete(sessions, name)
	out, err := json.MarshalIndent(sessions, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(sessionsPath, out, 0644)
}

func printSessions() {
	sessions, err := listSessions()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading sessions: %v\n", err)
		return
	}
	if len(sessions) == 0 {
		fmt.Println("No sessions found.")
		return
	}
	fmt.Println("Sessions:")
	keys := make([]string, 0, len(sessions))
	for k := range sessions {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, name := range keys {
		session := sessions[name]
		elapsedDur := parseFormattedDuration(session.Elapsed)
		var totalDur time.Duration
		if session.Mode == "timer" {
			remainingDur := parseFormattedDuration(session.Remaining)
			totalDur = elapsedDur + remainingDur
		} else {
			totalDur = elapsedDur
		}
		totalStr := formatDuration(totalDur)
		startStr := strings.Replace(session.Start, ":", " ", 1)
		fmt.Printf("%s: %s - Elapsed: %s - Total: %s - Paused: %t - Finished: %t - Started: %s - Inline: %t\n",
			name, session.Mode, session.Elapsed, totalStr, session.Paused, session.Finished, startStr, session.Inline)
	}
}
