package env

import (
	"strings"
	"testing"

	"github.com/jdarcyryan/conch/internal/manifest"
)

func loadFullExample(t *testing.T) *manifest.Manifest {
	t.Helper()
	m, err := manifest.Load("../../examples/08-full.toml")
	if err != nil {
		t.Fatalf("manifest load: %v", err)
	}
	return m
}

func TestActivateRendersPreferences(t *testing.T) {
	m := loadFullExample(t)
	l := New("/proj")
	out, err := l.ActivateContent(m)
	if err != nil {
		t.Fatal(err)
	}

	wantContains := []string{
		"Resolve-Path -ErrorAction Stop -LiteralPath (Join-Path $PSScriptRoot 'modules')",
		"Resolve-Path -ErrorAction Stop -LiteralPath (Join-Path $PSScriptRoot 'pwsh' 'Modules')",
		"$env:PSModulePath = $conchModules + [IO.Path]::PathSeparator + $pwshBundled",
		"$ErrorActionPreference = 'Stop'",
		"$WarningPreference = 'Continue'",
		"$VerbosePreference = 'SilentlyContinue'",
		"$ProgressPreference = 'SilentlyContinue'",
		"$ErrorView = 'ConciseView'",
		"$PSNativeCommandUseErrorActionPreference = $true",
	}
	for _, s := range wantContains {
		if !strings.Contains(out, s) {
			t.Errorf("activate.ps1 missing %q\nfull output:\n%s", s, out)
		}
	}
}

func TestActivateOmitsTasks(t *testing.T) {
	// Tasks must never make it into activate.ps1 — they're inlined
	// at run time so manifest edits take effect immediately, and
	// stale function definitions don't linger after a rename.
	m := loadFullExample(t)
	l := New("/proj")
	out, _ := l.ActivateContent(m)

	for _, leak := range []string{"conch_task_", "function ", "# Tasks"} {
		if strings.Contains(out, leak) {
			t.Errorf("activate.ps1 should not mention %q\nfull output:\n%s", leak, out)
		}
	}
}

func TestTaskScriptInlinesBody(t *testing.T) {
	l := New("/proj")
	got := l.TaskScript([]string{
		"Invoke-ScriptAnalyzer -Path . -Recurse -EnableExit",
		"Invoke-Pester -CI",
	})

	wantSourceLine := ". " + PSSingleQuote(l.ActivateScript())
	if !strings.HasPrefix(got, wantSourceLine+"\n") {
		t.Errorf("TaskScript should start by sourcing %s, got %q", wantSourceLine, got)
	}
	if !strings.Contains(got, "Invoke-ScriptAnalyzer -Path . -Recurse -EnableExit\nInvoke-Pester -CI") {
		t.Errorf("TaskScript should preserve every line of the task body, got %q", got)
	}
}

func TestActivateNoPreferences(t *testing.T) {
	m, err := manifest.Load("../../examples/01-minimal.toml")
	if err != nil {
		t.Fatal(err)
	}
	out, err := New("/proj").ActivateContent(m)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "# Preferences") {
		t.Errorf("expected no preferences section for minimal manifest")
	}
	if !strings.Contains(out, "$env:PSModulePath") {
		t.Errorf("PSModulePath should always be set, even for minimal manifests")
	}
	if !strings.Contains(out, "$PSScriptRoot") {
		t.Errorf("activation should anchor on $PSScriptRoot")
	}
}
