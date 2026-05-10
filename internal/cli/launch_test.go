package cli

import (
	"os"
	"strings"
	"testing"

	"github.com/jdarcyryan/conch/internal/env"
)

func TestWithConchEnvOverridesPSModulePath(t *testing.T) {
	layout := env.New("/proj")
	base := []string{
		"FOO=bar",
		"PSMODULEPATH=/old/modules:/other",
		"PATH=/usr/bin:/bin",
	}
	got := withConchEnv(base, layout)

	// Old PSModulePath must be gone.
	for _, kv := range got {
		if strings.HasPrefix(strings.ToUpper(kv), "PSMODULEPATH=/old/") {
			t.Errorf("host PSModulePath leaked: %q", kv)
		}
	}

	// New PSModulePath must list both the project's modules dir and
	// the bundled pwsh modules dir, in that order.
	want := layout.ModulesDir() + string(os.PathListSeparator) + layout.BundledModulesDir()
	if !contains(got, "PSModulePath="+want) {
		t.Errorf("expected PSModulePath=%s, full env: %v", want, got)
	}
}

func TestWithConchEnvPrependsPwshDirToPATH(t *testing.T) {
	layout := env.New("/proj")
	base := []string{"PATH=/usr/bin" + string(os.PathListSeparator) + "/bin"}

	got := withConchEnv(base, layout)
	pathLine := findEnv(got, "PATH")
	if pathLine == "" {
		t.Fatal("PATH not set")
	}
	want := layout.PowerShellDir() + string(os.PathListSeparator) + "/usr/bin" + string(os.PathListSeparator) + "/bin"
	if pathLine != want {
		t.Errorf("PATH = %q, want %q", pathLine, want)
	}
}

func TestWithConchEnvPreservesUnrelatedKeys(t *testing.T) {
	layout := env.New("/proj")
	base := []string{"FOO=bar", "USER=alice"}

	got := withConchEnv(base, layout)
	if !contains(got, "FOO=bar") {
		t.Errorf("dropped FOO=bar")
	}
	if !contains(got, "USER=alice") {
		t.Errorf("dropped USER=alice")
	}
}

func TestWithConchEnvSetsCONCH_ENV(t *testing.T) {
	layout := env.New("/some/proj")
	got := withConchEnv(nil, layout)
	if !contains(got, "CONCH_ENV=/some/proj") {
		t.Errorf("CONCH_ENV not set, env: %v", got)
	}
}

func contains(env []string, want string) bool {
	for _, kv := range env {
		if kv == want {
			return true
		}
	}
	return false
}

func findEnv(env []string, key string) string {
	prefix := key + "="
	for _, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			return strings.TrimPrefix(kv, prefix)
		}
	}
	return ""
}
