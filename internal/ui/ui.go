// Package ui owns user-facing output for conch CLI commands. It picks
// between two modes:
//
//   - TUI mode: a colourful single-line progress bar redrawn in place
//     with ANSI escape codes. Suitable for interactive terminals.
//   - Log mode: timestamped, level-prefixed lines (one per event).
//     Suitable for CI logs or piped output where carriage-return
//     redraws look like garbage.
//
// The mode is chosen by the CLI in this priority order:
//
//  1. Explicit --log / --tui flag
//  2. [output].mode in conch.toml ("tui", "log", or "auto")
//  3. Auto-detect: TUI if stdout is a TTY, otherwise log.
package ui

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Mode is the rendering style. ModeAuto defers the decision to the
// runtime — TTY-ness of stdout — at construction time.
type Mode int

const (
	ModeAuto Mode = iota
	ModeTUI
	ModeLog
)

// ParseMode decodes a TOML or flag value. Empty/unrecognised falls
// back to ModeAuto.
func ParseMode(s string) Mode {
	switch s {
	case "tui":
		return ModeTUI
	case "log":
		return ModeLog
	case "auto", "":
		return ModeAuto
	}
	return ModeAuto
}

// UI is the front-end shared by every conch command. Methods are safe
// for concurrent use; the embedded mutex serialises bar repaints, the
// spinner goroutine, and status messages so they never tear.
type UI struct {
	mode Mode
	out  io.Writer
	err  io.Writer
	mu   sync.Mutex
	now  func() time.Time

	// activeStep, if non-nil, is the spinner currently animating the
	// current line. Step starts one; Done / BeginDownload / line()
	// stop it before printing their own output.
	activeStep *spinningStep
}

// New returns a UI rendering to the given streams.
func New(mode Mode, stdout, stderr io.Writer) *UI {
	return &UI{mode: mode, out: stdout, err: stderr, now: time.Now}
}

// Auto picks ModeTUI when stdout is an interactive terminal, ModeLog
// otherwise.
func Auto(stdout, stderr *os.File) *UI {
	mode := ModeLog
	if IsTerminal(stdout) {
		mode = ModeTUI
	}
	return New(mode, stdout, stderr)
}

// Resolve returns a UI honouring an explicit mode where possible,
// falling back to the auto-detect rules. Used by the CLI to combine
// flag, toml, and TTY inputs.
func Resolve(explicit Mode, stdout, stderr *os.File) *UI {
	if explicit == ModeAuto {
		return Auto(stdout, stderr)
	}
	return New(explicit, stdout, stderr)
}

// Mode returns the resolved mode (never ModeAuto — that has been
// flattened to TUI or Log at construction time).
func (u *UI) Mode() Mode { return u.mode }

// Step announces a new in-progress operation.
//
//   - Log mode: prints one INFO-level line and returns.
//   - TUI mode: starts a spinner on the current line. The spinner
//     animates until Done finalises it (or until a download / warning
//     / error preempts it).
func (u *UI) Step(format string, args ...any) {
	msg := capitalise(fmt.Sprintf(format, args...))
	if u.mode == ModeLog {
		u.line("info", "", "%s", msg)
		return
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	u.startSpinner(msg)
}

// Done marks the currently in-progress Step as complete.
//
//   - Log mode: prints one INFO-level line.
//   - TUI mode: stops the spinner and replaces its line with a green
//     checkmark plus the supplied message.
func (u *UI) Done(format string, args ...any) {
	msg := capitalise(fmt.Sprintf(format, args...))
	if u.mode == ModeLog {
		u.line("info", "", "%s", msg)
		return
	}
	u.stopSpinner()
	u.mu.Lock()
	defer u.mu.Unlock()
	fmt.Fprint(u.out, ansiClearLine)
	fmt.Fprintf(u.out, "%s✓%s %s\n", ansiGreen, ansiReset, msg)
}

// Warn prints a warning. Stops any in-progress spinner first so the
// warning isn't immediately overwritten. Goes to stderr in log mode.
func (u *UI) Warn(format string, args ...any) {
	u.stopSpinner()
	u.line("warn", "! ", format, args...)
}

// Errorf prints an error. Stops any in-progress spinner first. Goes
// to stderr in log mode.
func (u *UI) Errorf(format string, args ...any) {
	u.stopSpinner()
	u.line("error", "✗ ", format, args...)
}

func (u *UI) line(level, prefix, format string, args ...any) {
	msg := capitalise(fmt.Sprintf(format, args...))
	u.mu.Lock()
	defer u.mu.Unlock()

	if u.mode == ModeTUI {
		// Clear any in-progress bar before printing.
		fmt.Fprint(u.out, ansiClearLine)
		// Warnings and errors get the whole line tinted; informational
		// lines tint just the prefix so longer message bodies stay
		// readable at any colour scheme.
		switch level {
		case "warn":
			fmt.Fprintln(u.out, ansiYellow+prefix+msg+ansiReset)
		case "error":
			fmt.Fprintln(u.out, ansiRed+prefix+msg+ansiReset)
		default:
			fmt.Fprintln(u.out, colourise(level, prefix)+msg+ansiReset)
		}
		return
	}

	dst := u.out
	if level == "warn" || level == "error" {
		dst = u.err
	}
	// Single space between every column — terse and machine-friendly.
	fmt.Fprintf(dst, "%s %s %s\n", u.now().UTC().Format("2006-01-02T15:04:05Z"), levelLabel(level), msg)
}

// capitalise upper-cases the first rune of s, leaving the rest
// untouched. Used so callers can keep Go-idiomatic lowercase format
// strings while output presents capitalised messages.
func capitalise(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	if r[0] >= 'a' && r[0] <= 'z' {
		r[0] -= 'a' - 'A'
	}
	return string(r)
}

// BeginDownload announces a streaming download. total may be -1 if the
// total size isn't known (no Content-Length header) — the bar
// degrades to a spinner-style "X.YMB" counter.
//
// Calling BeginDownload halts any in-progress Step spinner: the bar
// will redraw on the same line as the spinner had occupied, so the
// transition is seamless.
func (u *UI) BeginDownload(name string, total int64) *Download {
	u.stopSpinner()
	d := &Download{
		ui:      u,
		name:    name,
		total:   total,
		started: u.now(),
	}
	if u.mode == ModeLog {
		u.line("info", "", "downloading %s (%s)", name, fmtBytes(total))
	}
	return d
}

// --- internal helpers -------------------------------------------------------

func levelLabel(l string) string {
	switch l {
	case "info":
		return "INFO"
	case "warn":
		return "WARN"
	case "error":
		return "ERROR"
	}
	return l
}
