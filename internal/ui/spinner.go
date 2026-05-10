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
// can wait for it cleanly.
type spinningStep struct {
	msg  string
	stop chan struct{}
	done chan struct{}
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

	fmt.Fprint(u.out, renderSpinFrame(0, msg))
	go u.spin(s)
}

// renderSpinFrame composes one full spinner-line redraw into a single
// string so it can be flushed to the terminal in one Write call. Two
// separate writes (clear, then content) produce a visible blank-line
// state between them which the eye perceives as flicker. One write
// keeps the redraw atomic from the terminal's point of view.
func renderSpinFrame(frame int, msg string) string {
	return ansiClearLine + conchColour + spinFrames[frame] + ansiReset + " " + msg
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
// preempt cleanly. Each frame is one Write so the terminal never
// renders a half-cleared line.
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
			frame := renderSpinFrame(i, s.msg)
			u.mu.Lock()
			fmt.Fprint(u.out, frame)
			u.mu.Unlock()
			i = (i + 1) % len(spinFrames)
		}
	}
}
