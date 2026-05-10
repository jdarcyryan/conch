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

func TestActivateRendersTaskFunctions(t *testing.T) {
	m := loadFullExample(t)
	l := New("/proj")
	out, _ := l.ActivateContent(m)

	wantTasks := []string{"conch_task_test", "conch_task_lint", "conch_task_ci", "conch_task_release"}
	for _, name := range wantTasks {
		if !strings.Contains(out, "function "+name+" {") {
			t.Errorf("activate.ps1 missing task function %s", name)
		}
	}
	// Multi-line ci task should preserve its lines.
	if !strings.Contains(out, "Invoke-ScriptAnalyzer -Path . -Recurse -EnableExit") {
		t.Errorf("activate.ps1 missing ci task body")
	}
	if !strings.Contains(out, "Invoke-Pester -CI") {
		t.Errorf("activate.ps1 missing ci task second line")
	}
}

func TestActivateNoPreferencesNoTasks(t *testing.T) {
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
	if strings.Contains(out, "# Tasks") {
		t.Errorf("expected no tasks section for minimal manifest")
	}
	if !strings.Contains(out, "$env:PSModulePath") {
		t.Errorf("PSModulePath should always be set, even for minimal manifests")
	}
	if !strings.Contains(out, "$PSScriptRoot") {
		t.Errorf("activation should anchor on $PSScriptRoot")
	}
}
