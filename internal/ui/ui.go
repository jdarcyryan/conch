// Package ui owns user-facing output for conch CLI commands. It picks
// between two modes:
//
//   - TUI mode: a colourful single-line progress bar redrawn in place
//     with ANSI escape codes; when a Step is active and a download
//     starts, the spinner stays on its line and the bar paints on the
//     line beneath it (a single render frame redraws both). Suitable
//     for interactive terminals.
//   - Log mode: timestamped, level-prefixed lines (one per event).
//     Suitable for CI logs or piped output where carriage-return
//     redraws look like garbage.
//
// The mode is chosen by the CLI in this priority order:
//
//  1. Explicit --min-ui flag (forces ModeLog).
//  2. Auto-detect: TUI if stdout is a TTY, otherwise log.
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
	// top line of the live frame.
	//
	// activeDownload, if non-nil, is the streaming download whose
	// progress bar currently occupies the bottom line of the live
	// frame.
	//
	// The four (nil, nil), (step, nil), (nil, dl), (step, dl)
	// combinations all map cleanly through renderTUILocked — see its
	// comment for the cursor protocol that holds the redraws in
	// place.
	activeStep     *spinningStep
	activeDownload *Download
}

// newRenderer returns a UI rendering to the given streams. Constructor
// is package-private; external callers go through Resolve, which
// flattens ModeAuto into TUI or Log based on TTY detection.
func newRenderer(mode Mode, stdout, stderr io.Writer) *UI {
	return &UI{mode: mode, out: stdout, err: stderr, now: time.Now}
}

// Resolve returns a UI honouring an explicit mode where possible,
// falling back to TTY-based auto-detection. Used by the CLI to fold
// the --min-ui flag and stdout's shape into a single decision.
func Resolve(explicit Mode, stdout, stderr *os.File) *UI {
	if explicit == ModeAuto {
		mode := ModeLog
		if IsTerminal(stdout) {
			mode = ModeTUI
		}
		return newRenderer(mode, stdout, stderr)
	}
	return newRenderer(explicit, stdout, stderr)
}

// Step announces a new in-progress operation.
//
//   - Log mode: prints one INFO-level line and returns.
//   - TUI mode: starts a spinner on the top line of the live frame.
//     If a download is already running, the spinner takes the line
//     above the bar; otherwise it occupies the only line. The
//     spinner animates until Done finalises it.
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

// Warn prints a warning. Tears down any live frame first so the
// warning isn't immediately overwritten by the next spinner tick.
// Goes to stderr in log mode.
func (u *UI) Warn(format string, args ...any) {
	u.tearDownFrame()
	u.line("warn", "! ", format, args...)
}

// Errorf prints an error. Tears down any live frame first. Goes to
// stderr in log mode.
func (u *UI) Errorf(format string, args ...any) {
	u.tearDownFrame()
	u.line("error", "✗ ", format, args...)
}

// Fail abandons any live frame — spinner and/or download bar — and
// leaves the cursor at the start of a cleared line. Call it before
// returning an error that something outside the UI (cmd/conch's
// top-level error reporter) will print; without it the error appends
// to the spinner's unfinished line. No-op in log mode, where every
// line is already newline-terminated.
func (u *UI) Fail() {
	u.tearDownFrame()
	if u.mode != ModeTUI {
		return
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	// tearDownFrame leaves the cursor on the (still-painted) spinner
	// row; wipe it so the error starts on a clean line.
	fmt.Fprint(u.out, ansiClearLine)
}

// tearDownFrame stops the spinner and detaches any in-progress
// download so subsequent line() output isn't fighting either of them
// for the cursor.
func (u *UI) tearDownFrame() {
	u.stopSpinner()
	u.mu.Lock()
	if u.activeDownload != nil {
		// Clear the bar line and step the cursor up to where the
		// spinner had been; line() then ansiClearLines that and
		// writes its message there.
		if u.mode == ModeTUI {
			fmt.Fprint(u.out, ansiClearLine+ansiCursorUp)
		}
		u.activeDownload = nil
	}
	u.mu.Unlock()
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

// renderTUILocked redraws the live frame — spinner line, bar line,
// or both — based on the current activeStep / activeDownload state.
// Caller must hold u.mu.
//
// Cursor protocol the rest of the package depends on:
//
//   - Spinner-only frame: cursor ends at end of spinner line.
//   - Bar-only frame: cursor ends at end of bar line.
//   - Dual frame: cursor ends at end of bar line (the row below the
//     spinner). The redraw walks up to the spinner line, repaints
//     it, then walks back down and repaints the bar; \r\x1b[2K is
//     issued for each row so partial frame residue from a previous
//     wider message is wiped.
func (u *UI) renderTUILocked() {
	if u.mode != ModeTUI {
		return
	}
	switch {
	case u.activeStep != nil && u.activeDownload != nil:
		fmt.Fprint(u.out,
			ansiClearLine+ // clear bar line (cursor enters here)
				ansiCursorUp+ansiClearLine+ // step up to spinner line, clear it
				renderSpinFrame(u.activeStep.frame, u.activeStep.msg)+
				"\n"+ansiClearLine+ // advance to bar line, ensure clean
				u.activeDownload.renderBar())
	case u.activeStep != nil:
		fmt.Fprint(u.out,
			ansiClearLine+renderSpinFrame(u.activeStep.frame, u.activeStep.msg))
	case u.activeDownload != nil:
		fmt.Fprint(u.out,
			ansiClearLine+u.activeDownload.renderBar())
	}
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
// If a Step is currently running its spinner, the bar takes the row
// below it: BeginDownload writes a newline to advance the cursor to
// the bar's row, and from there the spinner ticker and the bar's
// Write both redraw a coherent two-row frame via renderTUILocked.
// Without a Step, the bar takes over the current line as before.
func (u *UI) BeginDownload(name string, total int64) *Download {
	d := &Download{
		ui:      u,
		name:    name,
		total:   total,
		started: u.now(),
	}
	if u.mode == ModeLog {
		u.line("info", "", "downloading %s (%s)", name, fmtBytes(total))
		return d
	}

	u.mu.Lock()
	defer u.mu.Unlock()
	u.activeDownload = d
	if u.activeStep != nil {
		// Open a fresh row beneath the spinner — the dual-frame
		// renderTUILocked expects the cursor on the bar row.
		fmt.Fprint(u.out, "\n")
	}
	u.renderTUILocked()
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
