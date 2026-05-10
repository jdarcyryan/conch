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

// Done finalises the download.
//
// TUI mode: the bar is cleared and nothing replaces it. The
// surrounding Step ("Installing PowerShell into ...") already told
// the user what was running; once the download is done that line
// disappears entirely so the next Step starts on a clean row.
//
// Log mode: a one-line summary with elapsed time and average speed,
// because there's no animated bar to clear and the line is the only
// signal the download finished at all.
func (d *Download) Done() {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.finished {
		return
	}
	d.finished = true

	switch d.ui.mode {
	case ModeTUI:
		d.ui.mu.Lock()
		fmt.Fprint(d.ui.out, ansiClearLine)
		d.ui.mu.Unlock()
	case ModeLog:
		elapsed := d.ui.now().Sub(d.started)
		speed := bytesPerSec(d.written, elapsed)
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

// renderBar formats one frame of the progress bar.
//
// Width is kept tight on purpose — about 45 visible columns at the
// busiest case ("[20-cell bar] 100% 999.9 MB/999.9 MB"). A wider
// frame wraps in a narrow terminal, and once it wraps, \r\x1b[2K
// only clears the line the cursor's on (the wrap remainder), leaving
// the first portion of the previous frame on screen — that's the
// "tiling bar" failure mode. 45 chars fits inside any 60-col+
// terminal without wrapping.
//
// The filename is deliberately omitted: the surrounding Step (e.g.
// "Installing PowerShell into …") already named the download, and a
// 30-char filename would push us straight back into wrap territory.
func (d *Download) renderBar() string {
	const width = 20
	written := d.written
	if d.total <= 0 {
		// Indeterminate: a single moving dot so the user knows
		// something's happening when Content-Length is missing.
		dots := strings.Repeat(" ", width)
		i := int(written/4096) % width
		dots = dots[:i] + "·" + dots[i+1:]
		return fmt.Sprintf("  [%s%s%s]  %s",
			ansiDim, dots, ansiReset, fmtBytes(written))
	}

	pct := float64(written) / float64(d.total)
	if pct > 1 {
		pct = 1
	}
	filled := int(pct * float64(width))
	bar := ansiGreen + strings.Repeat("█", filled) + ansiDim + strings.Repeat("░", width-filled) + ansiReset

	return fmt.Sprintf("  [%s] %3d%% %s/%s",
		bar, int(pct*100), fmtBytes(written), fmtBytes(d.total))
}

// bytesPerSec is used by Done's log-mode summary (TUI mode no longer
// shows speed inline so the bar doesn't need it).
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
