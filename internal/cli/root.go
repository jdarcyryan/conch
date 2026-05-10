// Package cli builds the cobra command tree exposed by cmd/conch.
//
// Commands here are intentionally thin: argument parsing and wiring,
// nothing more. Anything that has business logic worth testing lives
// in a sibling internal package.
package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/jdarcyryan/conch/internal/ui"
)

// minUI is set by `install --min-ui` (alias `-m`). It lives at package
// scope because cobra's flag binding is a one-time wire-up at command
// construction; the install RunE handler reads it after parsing.
var minUI bool

// New constructs the root cobra command. version is the only build
// metadata we surface — bare semver, no commit/date noise.
func New(version, commit, date string) *cobra.Command {
	_ = commit
	_ = date

	root := &cobra.Command{
		Use:           "conch",
		Short:         "Declarative PowerShell environment manager",
		Long:          "conch — describe a PowerShell environment in conch.toml and materialise it locally in a reproducible, isolated way.",
		SilenceUsage:  true,
		SilenceErrors: true,
		// Version flag is auto-registered on root only when this is set —
		// `conch list --version` etc. already error out as "unknown flag",
		// which is the behaviour we want.
		Version: version,
	}
	root.SetVersionTemplate("{{.Version}}\n")

	// Default `completion` subcommand cobra adds isn't useful for
	// conch right now and crowds the command list.
	root.CompletionOptions.DisableDefaultCmd = true

	root.AddCommand(
		newInitCmd(),
		newInstallCmd(),
		newShellCmd(),
		newRunCmd(),
		newSummaryCmd(),
		newTasksCmd(),
	)
	return root
}

// resolveUIMode returns the effective ui.Mode for one command
// invocation. Priority:
//
//  1. install/run/shell's --min-ui / -m flag forces ModeLog.
//  2. ui.ModeAuto, which ui.Resolve flattens to TUI or Log based on
//     stdout TTY-ness.
//
// Output mode is intentionally not configurable from conch.toml —
// rendering is a per-invocation choice, not a project property.
func resolveUIMode() ui.Mode {
	if minUI {
		return ui.ModeLog
	}
	return ui.ModeAuto
}

// newUI builds a UI honouring the --min-ui flag plus TTY detection.
// In TUI mode the conch ASCII banner is written to stdout once, here,
// so every command that goes through this constructor gets the same
// header. Log mode stays clean.
func newUI() *ui.UI {
	u := ui.Resolve(resolveUIMode(), os.Stdout, os.Stderr)
	u.Banner()
	return u
}
