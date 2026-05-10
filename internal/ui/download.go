package ui

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Download tracks one streaming transfer. It implements io.Writer:
// callers wrap an existing writer (the cache's temp file, the SHA
// hasher, etc.) with a MultiWriter so every Write also lands here for
// counting.
//
// Method calls are safe for concurrent use — Write and Done can race
// from different goroutines without scrambling the bar.
type Download struct {
	ui      *UI
	name    string
	total   int64
	started time.Time

	mu       sync.Mutex
	written  int64
	lastDraw time.Time
	finished bool
}

// Write implements io.Writer. It always reports n == len(p) and never
// returns an error; the bar is a passive observer.
func (d *Download) Write(p []byte) (int, error) {
	n := len(p)
	d.mu.Lock()
	d.written += int64(n)
	d.maybeDrawLocked()
	d.mu.Unlock()
	return n, nil
}

// Done finalises the download. In TUI mode it replaces the bar with a
// completion line; in log mode it prints a one-line summary with
// elapsed time and average speed.
func (d *Download) Done() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.finished {
		return
	}
	d.finished = true

	elapsed := d.ui.now().Sub(d.started)
	speed := bytesPerSec(d.written, elapsed)

	switch d.ui.mode {
	case ModeTUI:
		d.ui.mu.Lock()
		fmt.Fprint(d.ui.out, ansiClearLine)
		fmt.Fprintf(d.ui.out, "%s%s %s%s  %s in %s (%s/s)\n",
			ansiGreen, "✓", ansiReset, d.name, fmtBytes(d.written), fmtDuration(elapsed), fmtBytes(speed))
		d.ui.mu.Unlock()
	case ModeLog:
		d.ui.line("info", "✓ ", "downloaded %s (%s in %s, %s/s)",
			d.name, fmtBytes(d.written), fmtDuration(elapsed), fmtBytes(speed))
	}
}

// Fail finalises a download that errored out. The error is printed and
// the bar (if any) is cleared.
func (d *Download) Fail(err error) {
	d.mu.Lock()
	if d.finished {
		d.mu.Unlock()
		return
	}
	d.finished = true
	d.mu.Unlock()
	d.ui.Errorf("download %s failed: %v", d.name, err)
}

// maybeDrawLocked redraws the TUI bar at most ~10 Hz. The caller must
// hold d.mu.
func (d *Download) maybeDrawLocked() {
	if d.ui.mode != ModeTUI {
		return
	}
	now := d.ui.now()
	if !d.lastDraw.IsZero() && now.Sub(d.lastDraw) < 100*time.Millisecond {
		return
	}
	d.lastDraw = now

	d.ui.mu.Lock()
	defer d.ui.mu.Unlock()
	fmt.Fprint(d.ui.out, ansiClearLine+d.renderBar())
}

// renderBar formats one frame of the progress bar. Width is a fixed
// 30 cells — terminal-width detection without bringing in a TTY library
// is more trouble than it's worth, and 30 cells works comfortably in
// any reasonable terminal.
func (d *Download) renderBar() string {
	const width = 30
	written := d.written
	speed := bytesPerSec(written, d.ui.now().Sub(d.started))
	if d.total <= 0 {
		// Indeterminate: show a moving dot pattern so the user knows
		// progress is happening.
		dots := strings.Repeat(" ", width)
		i := int(written/4096) % width
		dots = dots[:i] + "·" + dots[i+1:]
		return fmt.Sprintf("%s%s%s [%s] %s  %s/s",
			ansiDim, "  ", ansiReset, dots, fmtBytes(written), fmtBytes(speed))
	}

	pct := float64(written) / float64(d.total)
	if pct > 1 {
		pct = 1
	}
	filled := int(pct * float64(width))
	bar := ansiGreen + strings.Repeat("█", filled) + ansiDim + strings.Repeat("░", width-filled) + ansiReset

	return fmt.Sprintf("  %s [%s] %3d%%  %s/%s  %s/s",
		truncate(d.name, 32), bar, int(pct*100),
		fmtBytes(written), fmtBytes(d.total), fmtBytes(speed))
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s + strings.Repeat(" ", max-len(s))
	}
	return s[:max-1] + "…"
}

func bytesPerSec(n int64, elapsed time.Duration) int64 {
	secs := elapsed.Seconds()
	if secs <= 0 {
		return 0
	}
	return int64(float64(n) / secs)
}

func fmtDuration(d time.Duration) string {
	switch {
	case d >= time.Minute:
		return fmt.Sprintf("%dm%02ds", int(d.Minutes()), int(d.Seconds())%60)
	case d >= time.Second:
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	return fmt.Sprintf("%dms", d.Milliseconds())
}
