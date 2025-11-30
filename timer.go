package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

// parseInput parses accumulated bytes into a key byte or ignores sequences
func parseInput(seq []byte) (byte, bool) {
	if len(seq) == 0 {
		return 0, false
	}
	if len(seq) == 1 {
		if seq[0] == 0x1b {
			return 0, false // Wait for more or timeout
		}
		// Single key
		return seq[0], true
	}
	// Check for complete escape sequences
	if seq[0] == 0x1b {
		// Mouse sequences: \033[M or \033[<...
		if len(seq) >= 3 && (seq[1] == '[' || seq[1] == 'M') {
			// Wait for end: for [ it's variable, for M it's 6 bytes
			if seq[1] == 'M' && len(seq) >= 6 {
				return 0, true // Ignore mouse
			}
			if seq[1] == '[' {
				// Extended mouse ends with 'm' or 'M'
				if seq[len(seq)-1] == 'm' || seq[len(seq)-1] == 'M' {
					return 0, true // Ignore mouse
				}
				// If not ended, continue accumulating
				return 0, false
			}
		}
		// Other escape sequences (e.g., arrow keys), ignore for now
		if len(seq) >= 3 && seq[len(seq)-1] >= 0x40 && seq[len(seq)-1] <= 0x7E {
			return 0, true // Ignore other escapes
		}
		// Incomplete, continue
		return 0, false
	}
	// Should not reach here, but if multiple bytes not starting with ESC, treat as single (though unlikely)
	return seq[0], true
}

// getTickerInterval returns the appropriate ticker interval based on duration
func getTickerInterval(duration time.Duration) time.Duration {
	if duration == 0 {
		// Counter mode - use fast interval for smooth display
		return tickIntervalFast
	}
	if duration < time.Minute {
		return tickIntervalFast
	}
	if duration < 10*time.Minute {
		return tickIntervalMedium
	}
	return tickIntervalSlow
}

func writeSession(session Session) {
	key := session.Name
	if key == "" {
		key = "default"
	}
	data, err := os.ReadFile("sessions.json")
	if err != nil && !os.IsNotExist(err) {
		return
	}
	sessions := make(map[string]Session)
	if err == nil {
		_ = json.Unmarshal(data, &sessions)
	}
	sessions[key] = session
	out, err := json.MarshalIndent(sessions, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile("sessions.json", out, 0644)
}

func runTimer(duration time.Duration, useFullscreen bool, initialPaused bool, name string, initialElapsed time.Duration, summaryCh chan<- TimerSummary) error {
	// Determine if counter mode (duration == 0)
	isCounter := duration == 0

	// Setup signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM, syscall.SIGWINCH)

	// Enter alt screen if fullscreen
	if useFullscreen {
		fmt.Print(altScreen)
		defer fmt.Print(mainScreen)
	}

	// Hide cursor
	fmt.Print(hideCursor)
	defer fmt.Print(showCursor)

	// Configure terminal for raw mode
	oldState, err := setupTerminal()
	if err != nil {
		return err
	}
	defer func() {
		if err := restoreTerminal(oldState); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to restore terminal: %v\n", err)
		}
	}()

	// Enable mouse tracking if fullscreen
	if useFullscreen {
		fmt.Print(mouseOn)
		defer fmt.Print(mouseOff)
	}

	// Channel for quit signal
	quitCh := make(chan struct{})
	defer close(quitCh)

	// Channel for keyboard input
	keysCh := make(chan byte, keyBufferSize)
	defer close(keysCh)

	// Start keyboard reader goroutine (blocking read, low CPU)
	fd := int(syscall.Stdin)
	go func() {
		var seq []byte
		var timer *time.Timer
		var timerCh <-chan time.Time
		for {
			buf := make([]byte, 1)
			readCh := make(chan []byte, 1)
			go func() {
				n, err := syscall.Read(fd, buf)
				if err != nil {
					readCh <- nil
					return
				}
				if n > 0 {
					readCh <- buf[:n]
				} else {
					readCh <- []byte{}
				}
			}()
			select {
			case data := <-readCh:
				if data == nil {
					// error
					if timer != nil {
						timer.Stop()
					}
					return
				}
				seq = append(seq, data[0])
				if timer != nil {
					timer.Stop()
					timer = nil
					timerCh = nil
				}
				if key, ok := parseInput(seq); ok {
					if key != 0 {
						select {
						case keysCh <- key:
						case <-quitCh:
							return
						default:
							// Drop key if channel is full
						}
					}
					seq = nil
				} else if len(seq) == 1 && seq[0] == 0x1b {
					// Start timer for ESC
					timer = time.NewTimer(50 * time.Millisecond)
					timerCh = timer.C
				}
			case <-timerCh:
				// Timeout, treat as ESC
				select {
				case keysCh <- 0x1b:
				case <-quitCh:
					return
				default:
				}
				seq = nil
				timer = nil
				timerCh = nil
			case <-quitCh:
				if timer != nil {
					timer.Stop()
				}
				return
			}
		}
	}()

	start := time.Now()
	if initialElapsed > 0 {
		start = start.Add(-initialElapsed)
	}
	// Use adaptive ticker interval based on duration
	tickInterval := getTickerInterval(duration)
	var ticker *time.Ticker = time.NewTicker(tickInterval)
	defer func() {
		if ticker != nil {
			ticker.Stop()
		}
	}()

	// Pause state
	var paused = initialPaused
	var pauseStart time.Time
	var totalPausedDuration time.Duration
	if paused {
		pauseStart = time.Now()
	}

	// Cache for rendered output
	var lastRenderedSec int64 = -1
	var cachedOutput string

	// Initial render - show the starting time immediately
	var initialDisplayTime time.Duration
	if isCounter {
		// Counter mode - start at 00:00
		initialDisplayTime = 0
	} else {
		// Timer mode - show full duration
		initialDisplayTime = duration
	}

	// Render initial state
	timeStr := formatHMS(initialDisplayTime)

	// Determine color based on state and time remaining
	var color string
	if paused {
		color = blueColor
	} else if !isCounter && initialDisplayTime < warningThreshold {
		// Only show red warning in timer mode
		color = redColor
	} else {
		color = ""
	}

	if useFullscreen {
		width, height := getTerminalSize()
		bigText := renderBigTime(timeStr, width, height)
		centeredText := centerText(bigText, width, height)
		if color != "" {
			cachedOutput = color + centeredText + resetStyle
		} else {
			cachedOutput = centeredText
		}
		fmt.Print(clearScreen + moveCursor(1, 1) + fixNewlines(cachedOutput))
	} else {
		if color != "" {
			cachedOutput = fmt.Sprintf("\r%s%s%s   ", color, timeStr, resetStyle)
		} else {
			cachedOutput = fmt.Sprintf("\r%s   ", timeStr)
		}
		fmt.Print(cachedOutput)
	}

	// Write initial session state
	var initialElapsedForDisplay time.Duration
	if isCounter {
		initialElapsedForDisplay = initialDisplayTime
	} else {
		initialElapsedForDisplay = initialElapsed
	}
	initialSession := Session{
		Start:    start.Format("2006-01-02:15-04-05"),
		Current:  start.Format("2006-01-02:15-04-05"),
		Elapsed:  formatDuration(initialElapsedForDisplay),
		Paused:   paused,
		Mode:     "timer",
		Name:     name,
		Finished: false,
		Inline:   !useFullscreen,
	}
	if isCounter {
		initialSession.Mode = "counter"
	} else {
		initialSession.Remaining = formatDuration(initialDisplayTime)
	}
	go writeSession(initialSession)

	lastRenderedSec = int64(initialDisplayTime.Seconds())

	for {
		select {
		case sig := <-sigCh:
			if sig == syscall.SIGWINCH {
				// Terminal resized - force re-render
				lastRenderedSec = -1
				continue
			}
			// Handle interrupt/terminate signals
			end := time.Now()
			effectiveDuration := time.Since(start) - totalPausedDuration
			if paused {
				effectiveDuration -= time.Since(pauseStart)
			}
			mode := "timer"
			if isCounter {
				mode = "counter"
			}
			// Write final session state
			signalSession := Session{
				Start:    start.Format("2006-01-02:15-04-05"),
				Current:  end.Format("2006-01-02:15-04-05"),
				Elapsed:  formatDuration(effectiveDuration),
				Paused:   paused,
				Mode:     mode,
				Name:     name,
				Finished: false,
				Inline:   !useFullscreen,
			}
			if !isCounter {
				remaining := duration - effectiveDuration
				if remaining < 0 {
					remaining = 0
				}
				signalSession.Remaining = formatDuration(remaining)
			}
			writeSession(signalSession) // Synchronous write for final state
			summaryCh <- TimerSummary{
				Start:    start,
				End:      end,
				Duration: effectiveDuration,
				Mode:     mode,
				Finished: false,
				Name:     name,
			}
			return nil

		case key := <-keysCh:
			// Handle keyboard input
			switch key {
			case 0x20: // Space key - pause/unpause
				if paused {
					// Unpause
					totalPausedDuration += time.Since(pauseStart)
					paused = false
					// Restart ticker with normal interval
					if ticker != nil {
						ticker.Stop()
					}
					ticker = time.NewTicker(tickInterval)
				} else {
					// Pause
					paused = true
					pauseStart = time.Now()
					// Switch to slow ticker to reduce CPU usage
					if ticker != nil {
						ticker.Stop()
					}
					ticker = time.NewTicker(tickIntervalSlow)
				}
				// Force re-render
				lastRenderedSec = -1

			case 'q', 'Q', 0x1b: // q, Q, or ESC - quit
				fmt.Print("\r\nquitting...\r\n")
				end := time.Now()
				effectiveDuration := time.Since(start) - totalPausedDuration
				if paused {
					effectiveDuration -= time.Since(pauseStart)
				}
				mode := "timer"
				if isCounter {
					mode = "counter"
				}
				// Write final session state
				quitSession := Session{
					Start:    start.Format("2006-01-02:15-04-05"),
					Current:  end.Format("2006-01-02:15-04-05"),
					Elapsed:  formatDuration(effectiveDuration),
					Paused:   paused,
					Mode:     mode,
					Name:     name,
					Finished: false,
					Inline:   !useFullscreen,
				}
				if !isCounter {
					remaining := duration - effectiveDuration
					if remaining < 0 {
						remaining = 0
					}
					quitSession.Remaining = formatDuration(remaining)
				}
				writeSession(quitSession) // Synchronous write for final state
				summaryCh <- TimerSummary{
					Start:    start,
					End:      end,
					Duration: effectiveDuration,
					Mode:     mode,
					Finished: false,
				}
				return nil

			case 0x03: // Ctrl+C
				end := time.Now()
				effectiveDuration := time.Since(start) - totalPausedDuration
				if paused {
					effectiveDuration -= time.Since(pauseStart)
				}
				mode := "timer"
				if isCounter {
					mode = "counter"
				}
				// Write final session state
				ctrlcSession := Session{
					Start:    start.Format("2006-01-02:15-04-05"),
					Current:  end.Format("2006-01-02:15-04-05"),
					Elapsed:  formatDuration(effectiveDuration),
					Paused:   paused,
					Mode:     mode,
					Name:     name,
					Finished: false,
					Inline:   !useFullscreen,
				}
				if !isCounter {
					remaining := duration - effectiveDuration
					if remaining < 0 {
						remaining = 0
					}
					ctrlcSession.Remaining = formatDuration(remaining)
				}
				writeSession(ctrlcSession) // Synchronous write for final state
				summaryCh <- TimerSummary{
					Start:    start,
					End:      end,
					Duration: effectiveDuration,
					Mode:     mode,
					Finished: false,
				}
				return nil
			}

		case <-ticker.C:
			// Calculate effective elapsed time (excluding paused duration)
			elapsed := time.Since(start) - totalPausedDuration
			if paused {
				elapsed -= time.Since(pauseStart)
			}

			var displayTime time.Duration
			var currentSec int64

			if isCounter {
				// Counter mode - count up
				displayTime = elapsed
				currentSec = int64(elapsed.Seconds())
				// Never exit automatically in counter mode
			} else {
				// Timer mode - count down
				if elapsed >= duration {
					// Timer finished
					fmt.Print("\r\nfinished!\r\n")
					end := time.Now()
					effectiveDuration := time.Since(start) - totalPausedDuration
					if paused {
						effectiveDuration -= time.Since(pauseStart)
					}
					// Write final session state
					finalSession := Session{
						Start:     start.Format("2006-01-02:15-04-05"),
						Current:   end.Format("2006-01-02:15-04-05"),
						Elapsed:   formatDuration(effectiveDuration),
						Remaining: formatDuration(0),
						Paused:    false,
						Mode:      "timer",
						Name:      name,
						Finished:  true,
						Inline:    !useFullscreen,
					}
					writeSession(finalSession) // Synchronous write for final state
					summaryCh <- TimerSummary{
						Start:    start,
						End:      end,
						Duration: effectiveDuration,
						Mode:     "timer",
						Finished: true,
						Name:     name,
					}
					if runtime.GOOS == "linux" {
						title := "Timer"
						if name != "" {
							title = name
						}
						exec.Command("notify-send", title, "Timer finished!").Run()
					}
					return nil
				}
				displayTime = duration - elapsed
				currentSec = int64(displayTime.Seconds())
			}

			// Re-render when second changes OR when paused state changes
			if currentSec != lastRenderedSec || lastRenderedSec == -1 {
				lastRenderedSec = currentSec

				// Write current session to file
				currentTime := time.Now()
				session := Session{
					Start:    start.Format("2006-01-02:15-04-05"),
					Current:  currentTime.Format("2006-01-02:15-04-05"),
					Elapsed:  formatDuration(elapsed),
					Paused:   paused,
					Mode:     "timer",
					Name:     name,
					Finished: false,
					Inline:   !useFullscreen,
				}
				if isCounter {
					session.Mode = "counter"
				} else {
					session.Remaining = formatDuration(displayTime)
				}
				go writeSession(session) // Write asynchronously to avoid blocking UI

				// Format time
				timeStr := formatHMS(displayTime)

				// Determine color based on state and time remaining
				var color string
				if paused {
					color = blueColor
				} else if !isCounter && displayTime < warningThreshold {
					// Only show red warning in timer mode
					color = redColor
				} else {
					color = ""
				}

				if useFullscreen {
					// Get terminal size
					width, height := getTerminalSize()

					// Render big text
					bigText := renderBigTime(timeStr, width, height)

					// Center the output first
					centeredText := centerText(bigText, width, height)

					// Apply color (paused = blue, <5min = red, else = default)
					if color != "" {
						cachedOutput = color + centeredText + resetStyle
					} else {
						cachedOutput = centeredText
					}
				} else {
					// Simple inline display
					if color != "" {
						cachedOutput = fmt.Sprintf("\r%s%s%s   ", color, timeStr, resetStyle)
					} else {
						cachedOutput = fmt.Sprintf("\r%s   ", timeStr)
					}
				}
			}

			// Output the cached rendering (fix newlines for raw mode)
			if useFullscreen {
				fmt.Print(clearScreen + moveCursor(1, 1) + fixNewlines(cachedOutput))
			} else {
				fmt.Print(cachedOutput)
			}
		}
	}
}
