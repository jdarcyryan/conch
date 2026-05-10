package ui

import (
	"fmt"
	"time"
)

// spinFrames is the braille-dot rotation used as the leading character
// of an in-progress Step in TUI mode. Eight frames at 80ms gives a
// brisk-but-readable cadence.
var spinFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

const spinFrameInterval = 80 * time.Millisecond

// spinningStep is one in-flight Step. The goroutine that animates it
// listens on stop and signals done when it has exited so finalisers
// can wait for it cleanly. frame is the index of the spinner glyph
// most recently committed; it lives on the step (rather than as a
// goroutine local) so renderTUILocked — which can be called from
// either the spinner ticker or a download Write — sees the same
// frame for the same redraw.
type spinningStep struct {
	msg   string
	frame int
	stop  chan struct{}
	done  chan struct{}
}

// startSpinner starts a goroutine that overwrites the current line
// with cycling spinner frames until stopSpinner is called. Caller
// must hold u.mu when invoking.
//
// Frame zero is drawn synchronously while the caller still holds the
// lock — that way Step's caller (and tests) see output immediately,
// not at the mercy of goroutine scheduling. Subsequent frames come
// from the ticker.
//
// If a spinner is already running it is stopped first — Step is
// always "begin a fresh in-progress line".
func (u *UI) startSpinner(msg string) {
	if u.activeStep != nil {
		u.stopSpinnerLocked()
	}
	s := &spinningStep{
		msg:  msg,
		stop: make(chan struct{}),
		done: make(chan struct{}),
	}
	u.activeStep = s

	// Initial paint goes through renderTUILocked so the dual-mode
	// case (Step called while a download is in flight) draws a
	// coherent frame from the start.
	u.renderTUILocked()
	go u.spin(s)
}

// renderSpinFrame returns the visible content of one spinner row —
// coloured glyph followed by the message. Cursor positioning and
// line clearing are the caller's responsibility (renderTUILocked
// emits ansiClearLine right before this is interpolated into the
// frame string), keeping the whole frame a single Write.
func renderSpinFrame(frame int, msg string) string {
	return conchColour + spinFrames[frame] + ansiReset + " " + msg
}

// stopSpinner halts any in-progress spinner without writing a final
// line. The cursor stays on the current line so the caller's next
// write replaces the last frame.
func (u *UI) stopSpinner() {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.stopSpinnerLocked()
}

func (u *UI) stopSpinnerLocked() {
	s := u.activeStep
	if s == nil {
		return
	}
	u.activeStep = nil
	// Release the lock around the goroutine handshake so the spin
	// loop can take it to flush its last frame and exit.
	u.mu.Unlock()
	close(s.stop)
	<-s.done
	u.mu.Lock()
	fmt.Fprint(u.out, ansiClearLine)
}

// spin animates the spinner until stop is closed. Holds u.mu only for
// the duration of each frame write so the foreground caller can
// preempt cleanly. Frame composition is delegated to
// renderTUILocked so dual-mode redraws (spinner + download bar) stay
// in lock-step with bar updates triggered from Download.Write.
func (u *UI) spin(s *spinningStep) {
	defer close(s.done)

	ticker := time.NewTicker(spinFrameInterval)
	defer ticker.Stop()

	// Frame 0 was drawn synchronously by startSpinner before this
	// goroutine launched, so begin animating from frame 1.
	i := 1
	for {
		select {
		case <-s.stop:
			return
		case <-ticker.C:
			u.mu.Lock()
			s.frame = i
			u.renderTUILocked()
			u.mu.Unlock()
			i = (i + 1) % len(spinFrames)
		}
	}
}
