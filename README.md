# ⏱️ Timer

A minimal, blazing-fast TUI countdown timer and stopwatch for the terminal. Built for efficiency with **<5MB memory usage**.

## ✨ Features

- ⏱️ **Countdown Timer** - Set durations with intuitive syntax (`5s`, `2m`, `1h`)
- ⏲️ **Stopwatch Mode** - Count up from 00:00 when no duration is specified
- 🖥️ **Fullscreen TUI** - Large ASCII art display with centered output
- 📟 **Inline Mode** - Compact display option for command-line use
- ⏸️ **Pause/Resume** - Pause and resume timers with spacebar
- 🎨 **Color Indicators** - Visual feedback (red warning <5min, blue when paused)
- ⚡ **Low Resource Usage** - Optimized adaptive ticker intervals
- ⌨️ **Simple Controls** - Intuitive keyboard shortcuts (pause, quit, etc.)

## 🚀 Installation

### Using Go Install

```bash
# Install latest stable version
go install github.com/Zihad550/go-timer@v0.1.0

# Or install latest (may show development version)
go install github.com/Zihad550/go-timer@latest
```

### Prebuilt Binary (Linux amd64, no Go required)

```bash
curl -sL https://github.com/Zihad550/go-timer/releases/latest/download/go-timer_linux_amd64.tar.gz | tar xz go-timer
install -Dm755 go-timer ~/.local/bin/go-timer
```

### Build from Source

```bash
git clone https://github.com/Zihad550/go-timer.git
cd go-timer
go build -o timer .
```

## 📖 Usage

### Basic Examples

```bash
# Stopwatch mode (counts up from 00:00)
timer

# Countdown timer - 5 seconds
timer 5

# Countdown timer - 2 minutes
timer 2m

# Countdown timer - 1 hour
timer 1h

# Inline mode (no fullscreen)
timer -i 30s

# Named timer (shows name in notification)
timer -session "Pomodoro Session" 25m

# Named timer with shorthand
timer -s "Pomodoro Session" 25m

# Display version
timer -version

# Start fresh instead of restoring a saved session
timer --reset

# Start a fresh named countdown
timer --reset -s "Pomodoro Session" 25m
```

### Duration Format

- **Numbers only**: Interpreted as seconds (e.g., `timer 60` = 60 seconds)
- **With units**: `s` (seconds), `m` (minutes), `h` (hours)
- **Examples**: `5s`, `90s`, `2m`, `1h30m`

### Command-Line Options

| Flag | Shorthand | Description |
|------|-----------|-------------|
| `--inline` | `-i` | Run in inline mode (disable fullscreen TUI) |
| `--version` | `-v` | Display version information |
| `--session` | `-s` | Name for the timer (shown in notifications, used for session key) |
| `--paused` | `-p` | Start timer in paused state |
| `--reset` | | Start fresh, ignoring saved sessions (overrides `--restore`) |
| `--sessions` | | List all sessions |

### Configuration File & Sessions

Timer supports optional configuration via a JSON file located at `~/.config/go-timer/config.json`. This allows customization of display settings, timing intervals, and other parameters. Timer also persists sessions in `~/.config/go-timer/sessions.json` (auto-created), which can be restored with `--restore` / `-r` or via auto-restore when enabled.

#### Config File Format (Go-style durations, comments here for docs only)

```json
{
  "tickIntervalFast": "100ms",
  "tickIntervalMedium": "500ms",
  "tickIntervalSlow": "1s",
  "warningThreshold": "5m",
  "glyphWidth": 8,
  "glyphHeight": 7,
  "glyphSpacing": 1,
  "keyBufferSize": 10,
  "defaultTermWidth": 80,
  "defaultTermHeight": 24,
  "restore": false
}
```

#### Configuration Options

- `tickIntervalFast` (duration): Update interval for timers < 1 minute (default: 100ms, range: 10ms-1s)
- `tickIntervalMedium` (duration): Update interval for timers 1-10 minutes (default: 500ms, range: 10ms-1s)
- `tickIntervalSlow` (duration): Update interval for timers > 10 minutes (default: 1s, range: 10ms-5s)
- `warningThreshold` (duration): Time remaining when warning color activates (default: 5m, range: 1m-1h)
- `glyphWidth` (int): Width of each ASCII character in display (default: 8, range: 1-20)
- `glyphHeight` (int): Height of each ASCII character in display (default: 7, range: 1-20)
- `glyphSpacing` (int): Spacing between characters (default: 1, range: 0-5)
- `keyBufferSize` (int): Size of keyboard input buffer (default: 10, range: 1-100)
- `defaultTermWidth` (int): Default terminal width fallback (default: 80, range: 1-1000)
- `defaultTermHeight` (int): Default terminal height fallback (default: 24, range: 1-1000)
- `restore` (bool): Auto-restore last session when no duration is specified (default: false)

#### Notes

- The config file is optional - timer uses built-in defaults if not present
- Command-line flags take precedence over config file settings
- Duration values use Go's time.ParseDuration format (e.g., "100ms", "5m", "1h")
- Invalid or missing config values fall back to defaults
- Config values outside acceptable ranges are ignored to prevent performance issues
- When `restore` is true and no duration is provided (and `--restore` not disabled), timer automatically restores the last session with its original display mode (inline or fullscreen)
- Command-line flags take precedence over restored session settings, allowing users to override saved behavior when restoring
- `--reset` skips both automatic and explicit restore. It starts a fresh stopwatch, or a fresh countdown when a duration is supplied, using the selected session name. The new timer saves to that session as usual; other sessions are preserved.

## ⌨️ Keyboard Controls & Notifications

| Key | Action |
|-----|--------|
| <kbd>Space</kbd> | Pause/Resume timer |
| <kbd>q</kbd> / <kbd>Q</kbd> / <kbd>ESC</kbd> | Quit |
| <kbd>Ctrl</kbd>+<kbd>C</kbd> | Force quit |

On Linux, when a countdown finishes, the app triggers a `notify-send` desktop notification (if available), using the timer name as the title when set.

## 🎨 Visual Indicators

- **Default** - Normal white/terminal color
- **🔴 Red** - Countdown timer with <5 minutes remaining
- **🔵 Blue** - Timer is paused

## ⚙️ Technical Details

### Architecture

- **Language**: Go 1.24.0+
- **Dependencies**: `golang.org/x/term`
- **Memory**: <5MB footprint
- **Performance**: Adaptive ticker intervals based on duration
  - Fast (100ms) - Durations <1 minute
  - Medium (500ms) - Durations 1-10 minutes
  - Slow (1s) - Durations >10 minutes

### Project Structure

```
timer/
├── main.go         # CLI entry point and argument parsing
├── timer.go        # Core timer logic and event loop
├── display.go      # Text formatting and rendering
├── terminal.go     # Terminal control and raw mode
├── config.go       # Configuration constants
├── glyphs.go       # ASCII art character definitions
└── utils.go        # Helper functions
```

## 🛠️ Development

### Requirements

- Go 1.24.0 or higher
- Unix-like terminal (Linux, macOS, WSL)

### Building

```bash
go build -o timer .
```

### Running Tests

```bash
go test ./...
```

## 📝 Examples

### Pomodoro Timer (25 minutes)
```bash
timer 25m
```

### Quick Break (5 minutes)
```bash
timer 5m
```

### Meeting Timer (1 hour)
```bash
timer 1h
```

### Track Work Session (Stopwatch)
```bash
timer
```

### Inline Timer for Scripts
```bash
timer -i 10s && echo "Task complete!"
```

## 🤝 Contributing

Contributions are welcome! Feel free to:

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

This project is open source. Check the LICENSE file for details.

## 💡 Tips

- Use **stopwatch mode** (no arguments) for tracking open-ended tasks
- Combine with shell commands: `timer 25m && notify-send "Break time!"`
- Perfect for Pomodoro technique, cooking, workouts, and more
- Runs entirely in the terminal—no GUI required

---

Made with ⚡ by developers who value simplicity and performance.
