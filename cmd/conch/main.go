// Command conch is the conch CLI entry point. It does nothing but build
// the cobra command tree and dispatch.
package main

import (
	"fmt"
	"os"

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
		msg := "✗ Error: " + err.Error()
		if ui.IsTerminal(os.Stderr) {
			const ansiRed = "\x1b[31m"
			const ansiReset = "\x1b[0m"
			msg = ansiRed + msg + ansiReset
		}
		fmt.Fprintln(os.Stderr, msg)
		os.Exit(1)
	}
}
