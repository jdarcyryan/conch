package ui

import (
	"fmt"
	"os"
)

// ANSI escape sequences used by the TUI renderer. Kept short and in
// one place so the bar code stays readable.
const (
	ansiReset      = "\x1b[0m"
	ansiBold       = "\x1b[1m"
	ansiDim        = "\x1b[2m"
	ansiGreen      = "\x1b[32m"
	ansiYellow     = "\x1b[33m"
	ansiRed        = "\x1b[31m"
	ansiCyan       = "\x1b[36m"
	ansiClearLine  = "\r\x1b[2K"
	ansiCursorUp   = "\x1b[A"
	ansiHideCursor = "\x1b[?25l"
	ansiShowCursor = "\x1b[?25h"
)

// colourise wraps prefix in a level-appropriate colour. The message
// itself is left uncoloured so it stays readable on light/dark themes.
func colourise(level, prefix string) string {
	switch level {
	case "info":
		return ansiCyan + prefix + ansiReset
	case "warn":
		return ansiYellow + prefix + ansiReset
	case "error":
		return ansiRed + prefix + ansiReset
	}
	return prefix
}

// IsTerminal reports whether f refers to an interactive terminal. Used
// by Auto() to decide between TUI and log output, and by callers
// outside this package (cmd/conch's top-level error reporter) that
// want to gate ANSI usage the same way.
//
// Implementation: stdlib only. A character device on the file's
// descriptor is a tight enough proxy for a TTY in practice — pipes,
// files, and CI log streams all return false.
func IsTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

// fmtBytes pretty-prints a byte count. Negative input renders as "?".
func fmtBytes(n int64) string {
	if n < 0 {
		return "?"
	}
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case n >= GB:
		return fmt.Sprintf("%.1f GB", float64(n)/float64(GB))
	case n >= MB:
		return fmt.Sprintf("%.1f MB", float64(n)/float64(MB))
	case n >= KB:
		return fmt.Sprintf("%.1f KB", float64(n)/float64(KB))
	default:
		return fmt.Sprintf("%d B", n)
	}
}
