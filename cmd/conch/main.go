// Command conch is the conch CLI entry point. It does nothing but build
// the cobra command tree, dispatch, and format any error coming back.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/jdarcyryan/conch/internal/cli"
	"github.com/jdarcyryan/conch/internal/ui"
)

// Populated by the linker via -ldflags. See goreleaser.yaml.
//
// commit and date are injected for build provenance but aren't surfaced
// in `conch --version` — keeping them here as named vars means the
// goreleaser -X overrides bind cleanly without an unused-symbol error.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	_ = commit
	_ = date

	if err := cli.New(version, commit, date).Execute(); err != nil {
		// Match the UI's Errorf line format: a red ✗ prefix, "Error:"
		// label, and the message. Tinting only happens when stderr is
		// an interactive terminal — pipes and log files would
		// otherwise capture literal escape codes.
		msg := "✗ Error: " + friendlyError(err)
		if ui.IsTerminal(os.Stderr) {
			const ansiRed = "\x1b[31m"
			const ansiReset = "\x1b[0m"
			msg = ansiRed + msg + ansiReset
		}
		fmt.Fprintln(os.Stderr, msg)
		os.Exit(1)
	}
}

// friendlyError rewrites cobra's terse "unknown command" message into
// something the user can act on. Two distinct failure modes share the
// underlying cobra error:
//
//   - The user typed a real-but-wrong command name. We append a
//     `--help` hint and pass the message through.
//   - A shell wrapper (PowerShell function, alias, etc.) joined what
//     should have been multiple args into one token. The unknown
//     token then contains a space or a leading `-`. We call this
//     out explicitly because no `--help` lookup will tell the user
//     why their args don't work.
//
// Any error that isn't a cobra unknown-command passes through
// verbatim.
func friendlyError(err error) string {
	s := err.Error()
	tok, ok := unknownCommandToken(s)
	if !ok {
		return s
	}
	if looksJoined(tok) {
		return fmt.Sprintf(
			"could not parse arguments — your shell appears to have joined them into one token (got %q). "+
				"Check any wrapper function or alias around `conch` and pass arguments through with @args / $args splatting, "+
				"not as a single \"$args\" string.",
			tok,
		)
	}
	return fmt.Sprintf("unknown command %q — run `conch --help` to see available commands", tok)
}

// looksJoined reports whether tok smells like several CLI arguments
// concatenated. A space inside means at least two would-be args were
// merged; a leading "-" means a flag drifted into the subcommand
// position.
func looksJoined(tok string) bool {
	return strings.ContainsRune(tok, ' ') || strings.HasPrefix(tok, "-")
}

// unknownCommandToken extracts the inner string from cobra's
// `unknown command "X" for "Y"` error template. Returns ok=false
// when the input doesn't match.
func unknownCommandToken(s string) (string, bool) {
	const prefix = `unknown command "`
	if !strings.HasPrefix(s, prefix) {
		return "", false
	}
	rest := s[len(prefix):]
	end := strings.IndexByte(rest, '"')
	if end <= 0 {
		return "", false
	}
	return rest[:end], true
}
