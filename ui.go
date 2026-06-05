package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbletea"
)

type model struct {
	sessions map[string]Session
	keys     []string // cached keys slice for consistent ordering
	cursor   int
	selected string
	view     string // "list", "create", "edit"
	form     formModel
}

type formModel struct {
	name     string
	mode     string
	duration string
	paused   bool
	inline   bool
	cursor   int
	vimMode  string // "normal" or "insert"
}

func initialModel() model {
	sessions, _ := listSessions()
	keys := make([]string, 0, len(sessions))
	for k := range sessions {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return model{
		sessions: sessions,
		keys:     keys,
		cursor:   0,
		view:     "list",
		form:     formModel{mode: "timer", vimMode: "normal"},
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.view {
		case "list":
			return m.updateList(msg)
		case "create", "edit":
			return m.updateForm(msg)
		}
	}
	return m, nil
}

func (m model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.keys)-1 {
			m.cursor++
		}
	case "enter":
		if m.cursor < len(m.keys) {
			m.selected = m.keys[m.cursor]
		}
		// Start timer
		fmt.Println("Starting session", m.selected)
		return m, tea.Quit
	case "c":
		m.view = "create"
		m.form = formModel{mode: "timer", vimMode: "normal"}
	case "e":
		if m.cursor < len(m.keys) {
			name := m.keys[m.cursor]
			session := m.sessions[name]
			m.selected = name
			m.view = "edit"
			m.form = formModel{
				name:     name,
				mode:     session.Mode,
				duration: session.Remaining,
				paused:   session.Paused,
				inline:   session.Inline,
				vimMode:  "normal",
			}
		}
	case "d":
		if m.cursor < len(m.keys) {
			name := m.keys[m.cursor]
			deleteSession(name)
			delete(m.sessions, name)
			// Rebuild keys slice after deletion
			m.keys = make([]string, 0, len(m.sessions))
			for k := range m.sessions {
				m.keys = append(m.keys, k)
			}
			sort.Strings(m.keys)
			// Adjust cursor if necessary
			if m.cursor >= len(m.keys) && len(m.keys) > 0 {
				m.cursor = len(m.keys) - 1
			} else if len(m.keys) == 0 {
				m.cursor = 0
			}
		}
	}
	return m, nil
}

func (m model) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.form.vimMode == "normal" {
		switch msg.String() {
		case "esc":
			m.view = "list"
		case "enter":
			if m.view == "create" {
				duration, _ := time.ParseDuration(m.form.duration)
				session := Session{
					Start:    time.Now().Format("2006-01-02:15-04-05"),
					Current:  time.Now().Format("2006-01-02:15-04-05"),
					Elapsed:  "0s",
					Paused:   m.form.paused,
					Mode:     m.form.mode,
					Name:     m.form.name,
					Finished: false,
					Inline:   m.form.inline,
				}
				if m.form.mode == "timer" {
					session.Remaining = formatDuration(duration)
				}
				saveSession(m.form.name, session)
				m.sessions[m.form.name] = session
				// Rebuild keys slice after adding session
				m.keys = make([]string, 0, len(m.sessions))
				for k := range m.sessions {
					m.keys = append(m.keys, k)
				}
				sort.Strings(m.keys)
			} else if m.view == "edit" {
				duration, _ := time.ParseDuration(m.form.duration)
				session := Session{
					Start:    time.Now().Format("2006-01-02:15-04-05"),
					Current:  time.Now().Format("2006-01-02:15-04-05"),
					Elapsed:  "0s",
					Paused:   m.form.paused,
					Mode:     m.form.mode,
					Name:     m.form.name,
					Finished: false,
					Inline:   m.form.inline,
				}
				if m.form.mode == "timer" {
					session.Remaining = formatDuration(duration)
				}
				saveSession(m.form.name, session)
				if m.form.name != m.selected {
					deleteSession(m.selected)
					delete(m.sessions, m.selected)
				}
				m.sessions[m.form.name] = session
				// Rebuild keys slice after editing session
				m.keys = make([]string, 0, len(m.sessions))
				for k := range m.sessions {
					m.keys = append(m.keys, k)
				}
				sort.Strings(m.keys)
			}
			m.view = "list"
		case "up", "k":
			if m.form.cursor > 0 {
				m.form.cursor--
			}
		case "down", "j":
			if m.form.cursor < 4 {
				m.form.cursor++
			}
		case " ":
			switch m.form.cursor {
			case 1:
				if m.form.mode == "timer" {
					m.form.mode = "counter"
				} else {
					m.form.mode = "timer"
				}
			case 3:
				m.form.paused = !m.form.paused
			case 4:
				m.form.inline = !m.form.inline
			}
		case "i", "a":
			m.form.vimMode = "insert"
		}
	} else { // insert mode
		switch msg.String() {
		case "esc":
			m.form.vimMode = "normal"
		default:
			switch m.form.cursor {
			case 0:
				m.form.name += msg.String()
			case 2:
				m.form.duration += msg.String()
			case 3: // Paused field
				switch strings.ToLower(msg.String()) {
				case "y", "yes", "true", "1":
					m.form.paused = true
				case "n", "no", "false", "0":
					m.form.paused = false
				}
			case 4: // Inline field
				switch strings.ToLower(msg.String()) {
				case "y", "yes", "true", "1":
					m.form.inline = true
				case "n", "no", "false", "0":
					m.form.inline = false
				}
			}
		}
	}
	return m, nil
}

func (m model) View() string {
	switch m.view {
	case "list":
		return m.viewList()
	case "create", "edit":
		return m.viewForm()
	}
	return ""
}

func (m model) viewList() string {
	var b strings.Builder
	b.WriteString("Go Timer Sessions\n\n")
	for i, name := range m.keys {
		cursor := " "
		if m.cursor == i {
			cursor = ">"
		}
		session := m.sessions[name]
		b.WriteString(fmt.Sprintf("%s %s: %s - Elapsed: %s - Paused: %t\n", cursor, name, session.Mode, session.Elapsed, session.Paused))
	}
	b.WriteString("\n(enter) Start | (c) Create | (e) Edit | (d) Delete | (q) Quit\n")
	return b.String()
}

func (m model) viewForm() string {
	var b strings.Builder
	if m.view == "create" {
		b.WriteString("Create New Session\n\n")
	} else {
		b.WriteString("Edit Session\n\n")
	}
	fields := []string{"Name", "Mode", "Duration", "Paused", "Inline"}
	for i, field := range fields {
		cursor := " "
		if m.form.cursor == i {
			cursor = ">"
		}
		switch i {
		case 0:
			b.WriteString(fmt.Sprintf("%s %s: %s\n", cursor, field, m.form.name))
		case 1:
			b.WriteString(fmt.Sprintf("%s %s: %s\n", cursor, field, m.form.mode))
		case 2:
			b.WriteString(fmt.Sprintf("%s %s: %s\n", cursor, field, m.form.duration))
		case 3:
			if m.form.vimMode == "insert" {
				b.WriteString(fmt.Sprintf("%s %s: [y/n]\n", cursor, field))
			} else {
				paused := "No"
				if m.form.paused {
					paused = "Yes"
				}
				b.WriteString(fmt.Sprintf("%s %s: %s\n", cursor, field, paused))
			}
		case 4:
			if m.form.vimMode == "insert" {
				b.WriteString(fmt.Sprintf("%s %s: [y/n]\n", cursor, field))
			} else {
				inline := "No"
				if m.form.inline {
					inline = "Yes"
				}
				b.WriteString(fmt.Sprintf("%s %s: %s\n", cursor, field, inline))
			}
		}
	}
	b.WriteString("\n")
	if m.form.vimMode == "normal" {
		b.WriteString("-- NORMAL -- (i) Insert | (enter) Save | (esc) Cancel\n")
	} else {
		b.WriteString("-- INSERT -- (esc) Normal | (enter) Save\n")
	}
	return b.String()
}

func runUI() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
