package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/jdarcyryan/conch/internal/pwsh"
	"github.com/jdarcyryan/conch/internal/version"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Write a starter conch.toml in the current directory",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit()
		},
	}
}

func runInit() error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("locate working directory: %w", err)
	}
	path := filepath.Join(cwd, "conch.toml")

	// Refuse to overwrite. Git history is the right place to recover
	// a previous conch.toml — this command should never silently lose
	// work.
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("conch.toml already exists at %s — refusing to overwrite", path)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat %s: %w", path, err)
	}

	psVersion, err := latestPowerShellVersion()
	if err != nil {
		// Keep init useful offline: fall back to a known-good pin and
		// leave a comment explaining why.
		fmt.Fprintf(os.Stderr,
			"warning: could not fetch latest PowerShell from GitHub (%v) — using fallback\n",
			err)
		psVersion = fallbackPowerShellVersion
	}

	body := starterManifest(filepath.Base(cwd), psVersion)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	fmt.Printf("wrote %s\n", path)
	fmt.Println("next: run `conch install` to materialise the environment")
	return nil
}

// fallbackPowerShellVersion is the version embedded in starter
// manifests when GitHub is unreachable. Bump it when bumping support
// matrix; the live `latest` lookup is the normal path.
const fallbackPowerShellVersion = "7.5.6"

func latestPowerShellVersion() (string, error) {
	r := pwsh.Resolver{}
	rel, err := r.Resolve(mustSpec("*"))
	if err != nil {
		return "", err
	}
	return rel.Version.String(), nil
}

func mustSpec(s string) version.Spec {
	spec, err := version.ParseSpec(s)
	if err != nil {
		// "*" parses unconditionally — if this ever fires we have a
		// programming error in the version package.
		panic("invalid spec: " + s)
	}
	return spec
}

// starterManifest generates a conch.toml with every supported field
// represented as a comment, organised into the same sections users
// will eventually fill in. Filled-in values are kept to the minimum a
// `conch install` needs to succeed: project name, latest PowerShell.
func starterManifest(name, psVersion string) string {
	return fmt.Sprintf(starterTemplate, name, psVersion)
}

const starterTemplate = `# conch.toml — declarative PowerShell environment manifest.
#
# Every field below the required pair is commented out. Uncomment and
# adjust as you need. Examples for richer setups live in conch's
# examples/ directory.

[project]
name = %q

# version     = "0.1.0"
# description = "What this project is for."
# authors     = ["Jane Doe <jane@example.com>"]

# Restrict which OS/arch combinations this project supports. When
# omitted, every platform conch supports is allowed:
#   "windows-amd64", "windows-arm64", "linux-amd64", "linux-arm64"
# platforms = ["windows-amd64", "linux-amd64"]


[powershell]
# Filled in by 'conch init' from the latest GitHub release. Pin to an
# exact version, a wildcard line ("7.5.*"), a range (">=7.5,<8"), or
# the floating "*" / "latest".
version = %q


# [modules]
# Pester           = "5.7.1"
# PSScriptAnalyzer = "1.25.0"
# "Az.Accounts"    = ">=5.1.1,<6"
# "PSReadLine"     = "latest"


# [tasks]
# A task value is one of:
#   1. single-line string  — runs as a single PowerShell command;
#   2. multi-line string   — runs as a script ("""…""");
#   3. array of strings    — joined with newlines, runs as a script.
# test    = "Invoke-Pester"
# lint    = "Invoke-ScriptAnalyzer -Path . -Recurse"
# ci      = """
# Invoke-ScriptAnalyzer -Path . -Recurse -EnableExit
# Invoke-Pester -CI
# """
# release = [
#     "Invoke-Pester -CI",
#     "./build.ps1",
#     "./publish.ps1",
# ]


# [preferences]
# # Action preferences accept: "Continue", "SilentlyContinue", "Stop",
# # "Inquire", "Ignore", "Break".
# error              = "Stop"               # $ErrorActionPreference
# warning            = "Continue"           # $WarningPreference
# verbose            = "SilentlyContinue"   # $VerbosePreference
# debug              = "SilentlyContinue"   # $DebugPreference
# information        = "SilentlyContinue"   # $InformationPreference
# progress           = "Continue"           # $ProgressPreference (use SilentlyContinue to speed up scripts)
#
# confirm            = "High"               # "None" / "Low" / "Medium" / "High"
# error-view         = "ConciseView"        # "NormalView" / "CategoryView" / "ConciseView" / "DetailedView"
# module-autoload    = "All"                # "None" / "ModuleQualified" / "All"
# native-arg-passing = "Standard"           # "Legacy" / "Standard" / "Windows"
#
# whatif              = false               # $WhatIfPreference
# native-error-action = false               # $PSNativeCommandUseErrorActionPreference  (PS 7.3+)
#
# format-enum-limit  = 4                    # $FormatEnumerationLimit
# max-history        = 4096                 # $MaximumHistoryCount  (1–32768)
# ofs                = " "                  # $OFS  (output field separator)
#
# # Session log toggles (booleans). Off by default.
# log-command-health    = false
# log-command-lifecycle = false
# log-engine-health     = true
# log-engine-lifecycle  = true
# log-provider-health   = true
# log-provider-lifecycle = false
`
