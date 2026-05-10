package ui

import "fmt"

// conchColour is the brand-foreground 24-bit ANSI sequence — hex
// #69b5e6, rgb(105, 181, 230). Only emitted in TUI mode; log mode
// stays plain.
const conchColour = "\x1b[38;2;105;181;230m"

// banner is the conch ASCII art shown at the top of TUI sessions. No
// trailing newline — Banner adds the surrounding blank lines itself,
// which keeps the ANSI reset adjacent to the last glyph rather than
// stranded after a stray "\n".
const banner = ` ██████╗  ██████╗  ███╗   ██╗  ██████╗ ██╗  ██╗
██╔════╝ ██╔═══██╗ ████╗  ██║ ██╔════╝ ██║  ██║
██║      ██║>_ ██║ ██╔██╗ ██║ ██║      ███████║
██║      ██║   ██║ ██║╚██╗██║ ██║      ██╔══██║
╚██████╗ ╚██████╔╝ ██║ ╚████║ ╚██████╗ ██║  ██║
 ╚═════╝  ╚═════╝  ╚═╝  ╚═══╝  ╚═════╝ ╚═╝  ╚═╝`

// Banner writes the conch ASCII banner in the brand colour, with one
// blank line before and one after. No-op in log mode — banners would
// just be noise in CI logs, which is the whole reason log mode exists.
func (u *UI) Banner() {
	if u.mode != ModeTUI {
		return
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	fmt.Fprintf(u.out, "\n%s%s%s\n\n", conchColour, banner, ansiReset)
}
