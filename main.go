package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

var (
	inlineMode   = flag.Bool("inline", false, "run in inline mode (disable fullscreen TUI)")
	inlineModeS  = flag.Bool("i", false, "run in inline mode (shorthand for -inline)")
	showVersion  = flag.Bool("version", false, "display version information")
	showVersionS = flag.Bool("v", false, "display version information (shorthand for -version)")
	pausedMode   = flag.Bool("paused", false, "start timer in paused state")
	pausedModeS  = flag.Bool("p", false, "start timer in paused state (shorthand for -paused)")
	timerName    = flag.String("session", "", "name for the timer")
	timerNameS   = flag.String("s", "", "name for the timer (shorthand for -session)")
	restoreMode  = flag.Bool("restore", false, "restore timer from sessions.json")
	restoreModeS = flag.Bool("r", false, "restore timer from sessions.json (shorthand)")
	resetMode    = flag.Bool("reset", false, "start fresh, ignoring saved sessions (overrides -restore)")
	uiMode       = flag.Bool("ui", false, "open UI for managing sessions")
	uiModeS      = flag.Bool("u", false, "open UI for managing sessions (shorthand)")
	showSessions = flag.Bool("sessions", false, "list all sessions")
)

func usage() {
	fmt.Fprintf(os.Stderr, "timer - minimal tui countdown/timer app under 5mb memory usage \n\n")
	fmt.Fprintf(os.Stderr, "Usage: timer [options] [<duration>]\n\n")
	fmt.Fprintf(os.Stderr, "Duration: number with unit (5s, 2m, 1h). No unit defaults to seconds.\n")
	fmt.Fprintf(os.Stderr, "          If omitted, runs as a counter (stopwatch) counting up from 00:00.\n\n")
	fmt.Fprintf(os.Stderr, "Options:\n")
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "  -ui, -u       open UI for managing sessions\n")
	fmt.Fprintf(os.Stderr, "  -session, -s  name for the timer\n")
	fmt.Fprintf(os.Stderr, "  -sessions     list all sessions\n")
	fmt.Fprintf(os.Stderr, "\nExamples:\n")
	fmt.Fprintf(os.Stderr, "  timer                    # counter/stopwatch (counts up)\n")
	fmt.Fprintf(os.Stderr, "  timer 5                  # 5 seconds countdown (fullscreen)\n")
	fmt.Fprintf(os.Stderr, "  timer 2m                 # 2 minutes countdown (fullscreen)\n")
	fmt.Fprintf(os.Stderr, "  timer -i 30s             # inline mode countdown\n")
	fmt.Fprintf(os.Stderr, "  timer -p 5m              # 5 minutes countdown starting paused\n")
	fmt.Fprintf(os.Stderr, "  timer -session \"Pomodoro\" 25m  # named timer with notification\n")
	fmt.Fprintf(os.Stderr, "  timer -s \"Pomodoro\" 25m              # named timer with notification (shorthand)\n")
	fmt.Fprintf(os.Stderr, "  timer --restore                # restore \"default\" session from sessions.json\n")
	fmt.Fprintf(os.Stderr, "  timer --restore --session NAME    # restore session named NAME\n")
	fmt.Fprintf(os.Stderr, "  timer --restore -s NAME             # restore session named NAME\n")
	fmt.Fprintf(os.Stderr, "  timer --restore -i             # restore in inline mode regardless of saved setting\n")
	fmt.Fprintf(os.Stderr, "  timer --reset                  # start a fresh stopwatch without restoring\n")
	fmt.Fprintf(os.Stderr, "  timer --reset -s NAME 25m       # start a fresh named countdown\n")
	fmt.Fprintf(os.Stderr, "  timer -sessions                # list all saved sessions\n")
}

func main() {
	flag.Usage = usage
	flag.Parse()

	// Merge short/long flags for session name
	if *timerNameS != "" {
		*timerName = *timerNameS
	}

	// Load configuration from ~/.config/go-timer/config.json
	loadConfig()

	// Handle sessions list
	if *showSessions {
		printSessions()
		return
	}

	// Get args after initial flag parse
	args := flag.Args()

	// Separate flags and positional from remaining args
	var positional []string
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			if arg == "-i" || arg == "-inline" {
				*inlineMode = true
			} else if arg == "-p" || arg == "-paused" {
				*pausedMode = true
			} else if arg == "-v" || arg == "-version" {
				*showVersion = true
			} else if arg == "-reset" || arg == "--reset" {
				*resetMode = true
			} else {
				usage()
				os.Exit(1)
			}
		} else {
			positional = append(positional, arg)
		}
	}

	// Handle version
	if *showVersion || *showVersionS {
		fmt.Println("timer version", version)
		return
	}

	// Handle UI mode
	if *uiMode || *uiModeS {
		runUI()
		return
	}

	// Accept 0 or 1 positional arg
	if len(positional) > 1 {
		usage()
		os.Exit(1)
	}

	// Parse duration (0 means counter mode)
	var duration time.Duration
	if len(positional) == 0 {
		// Counter mode - use 0 duration as signal
		duration = 0
	} else {
		durStr := positional[0]
		addSuffixIfArgIsNumber(&durStr, "s")
		var err error
		duration, err = time.ParseDuration(durStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: invalid duration %q\n", positional[0])
			os.Exit(1)
		}
	}

	// Handle restore mode (manual or auto)
	isRestore := (*restoreMode || *restoreModeS) && !*resetMode
	var restoredSession Session
	if isRestore {
		var err error
		restoredSession, err = loadSession(*timerName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	} else if duration == 0 && restoreEnabled && !*resetMode {
		// Auto-restore if no duration specified and config has restore=true;
		// fall back to a fresh counter when there is no session to restore
		if s, err := loadSession(*timerName); err == nil {
			restoredSession = s
			isRestore = true
		}
	}
	var initialElapsed time.Duration
	if isRestore {
		// Override parameters from session
		if restoredSession.Mode == "counter" {
			duration = 0
			initialElapsed = parseFormattedDuration(restoredSession.Elapsed)
		} else {
			elapsed := parseFormattedDuration(restoredSession.Elapsed)
			remaining := parseFormattedDuration(restoredSession.Remaining)
			duration = elapsed + remaining
			initialElapsed = elapsed
		}
		if *timerName == "" {
			*timerName = restoredSession.Name
		}
	}

	// Merge short/long flags - fullscreen is default, inline disables it
	userProvidedInline := *inlineMode || *inlineModeS
	userProvidedPaused := *pausedMode || *pausedModeS

	useInline := userProvidedInline
	initialPaused := userProvidedPaused

	if isRestore {
		if !userProvidedPaused {
			initialPaused = restoredSession.Paused
		}
		if !userProvidedInline {
			useInline = restoredSession.Inline
		}
	}

	// Channel for timer summary
	summaryCh := make(chan TimerSummary, 1)

	// Run timer (fullscreen unless inline flag is set)
	if err := runTimer(duration, !useInline, initialPaused, *timerName, initialElapsed, summaryCh); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Receive and print summary
	summary := <-summaryCh
	if summary.Name != "" {
		fmt.Printf("Name: %s\n", summary.Name)
	}
	fmt.Printf("Start: %s\n", summary.Start.Format("2006-01-02 15:04:05"))
	fmt.Printf("End: %s\n", summary.End.Format("2006-01-02 15:04:05"))
	fmt.Printf("Duration: %s\n", summary.Duration)
	fmt.Printf("Mode: %s\n", summary.Mode)
	fmt.Printf("Finished: %t\n", summary.Finished)
}
