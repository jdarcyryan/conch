// Package manifest defines the conch.toml schema and a parser for it.
//
// The schema is driven by the files in examples/, which are the source
// of truth. Anything that doesn't appear in those files is rejected as
// an unknown key — typos in field names should be caught at parse time,
// not silently ignored.
package manifest

import (
	"fmt"

	"github.com/jdarcyryan/conch/internal/platform"
	"github.com/jdarcyryan/conch/internal/version"
)

// Manifest is a fully parsed and validated conch.toml.
type Manifest struct {
	Project     Project
	PowerShell  PowerShell
	Modules     []Module
	Tasks       []Task
	Preferences Preferences
	Output      Output
}

// Output controls how conch's CLI commands render progress and status.
// Empty values mean "auto-detect" — see internal/ui.ModeAuto.
type Output struct {
	// Mode is one of "tui", "log", or "auto" (or empty, treated as
	// "auto"). Anything else is rejected at parse time.
	Mode string `toml:"mode"`
}

// Project holds the [project] table.
type Project struct {
	Name        string
	Version     string
	Description string
	Authors     []string
	// Platforms is the list of OS/arch combinations this project
	// supports. When the manifest omits the field entirely, this is
	// every platform conch supports — see platform.All().
	Platforms []platform.Platform
}

// PowerShell holds the [powershell] table.
type PowerShell struct {
	Version version.Spec
}

// Module is one entry from [modules]. The order matches the order of
// keys in the source TOML; downstream code that needs deterministic
// behaviour (e.g. lockfile writing) should sort by Name.
type Module struct {
	Name string
	Spec version.Spec
}

// Task is one entry from [tasks]. Lines is the list of script lines —
// a single-string task collapses to one element, an array task keeps
// each element as its own line, and a multi-line string is split on
// newlines.
type Task struct {
	Name  string
	Lines []string
}

// SupportsPlatform reports whether p is in the manifest's supported
// platform set.
func (m Manifest) SupportsPlatform(p platform.Platform) bool {
	for _, q := range m.Project.Platforms {
		if p == q {
			return true
		}
	}
	return false
}

// validate runs cross-field checks once decoding has produced a
// well-typed Manifest.
func (m *Manifest) validate() error {
	if m.Project.Name == "" {
		return fmt.Errorf("[project].name is required")
	}
	if m.PowerShell.Version.Raw() == "" {
		return fmt.Errorf("[powershell].version is required")
	}
	if len(m.Project.Platforms) == 0 {
		m.Project.Platforms = platform.All()
	}
	if err := m.Output.validate(); err != nil {
		return err
	}
	return nil
}

func (o Output) validate() error {
	switch o.Mode {
	case "", "auto", "tui", "log":
		return nil
	}
	return fmt.Errorf("[output].mode = %q: must be one of \"tui\", \"log\", \"auto\"", o.Mode)
}
