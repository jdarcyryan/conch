package manifest

import (
	"path/filepath"
	"testing"

	"github.com/jdarcyryan/conch/internal/platform"
)

const examplesDir = "../../examples"

// All examples must round-trip through the parser cleanly. This is the
// schema-conformance contract called out in CLAUDE.md.
func TestExamplesParse(t *testing.T) {
	files := []string{
		"01-minimal.toml",
		"02-basic.toml",
		"03-version-specifiers.toml",
		"04-tasks.toml",
		"05-platform-restricted-win.toml",
		"06-platform-restricted-linux.toml",
		"07-preferences.toml",
		"08-full.toml",
		"09-output.toml",
	}
	for _, f := range files {
		t.Run(f, func(t *testing.T) {
			path := filepath.Join(examplesDir, f)
			if _, err := Load(path); err != nil {
				t.Fatalf("Load(%s): %v", path, err)
			}
		})
	}
}

func TestMinimal(t *testing.T) {
	m, err := Load(filepath.Join(examplesDir, "01-minimal.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if m.Project.Name != "minimal" {
		t.Errorf("name = %q, want minimal", m.Project.Name)
	}
	// No platforms declared — must default to all four.
	if got := len(m.Project.Platforms); got != 4 {
		t.Errorf("platforms count = %d, want 4 (default)", got)
	}
	if len(m.Modules) != 0 {
		t.Errorf("modules = %d, want 0", len(m.Modules))
	}
}

func TestPlatformRestrictedWindows(t *testing.T) {
	m, err := Load(filepath.Join(examplesDir, "05-platform-restricted-win.toml"))
	if err != nil {
		t.Fatal(err)
	}
	want := []platform.Platform{
		{OS: platform.Windows, Arch: platform.AMD64},
		{OS: platform.Windows, Arch: platform.ARM64},
	}
	if len(m.Project.Platforms) != len(want) {
		t.Fatalf("platforms count = %d, want %d", len(m.Project.Platforms), len(want))
	}
	for i, p := range want {
		if m.Project.Platforms[i] != p {
			t.Errorf("platforms[%d] = %v, want %v", i, m.Project.Platforms[i], p)
		}
	}
	if !m.SupportsPlatform(platform.Platform{OS: platform.Windows, Arch: platform.AMD64}) {
		t.Error("manifest should support windows-amd64")
	}
	if m.SupportsPlatform(platform.Platform{OS: platform.Linux, Arch: platform.AMD64}) {
		t.Error("manifest should NOT support linux-amd64")
	}
}

func TestTasksFormsAllParse(t *testing.T) {
	m, err := Load(filepath.Join(examplesDir, "04-tasks.toml"))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{
		"test":    1, // single-line
		"lint":    1, // single-line
		"ci":      2, // multi-line """ """ — 2 commands
		"release": 4, // array
	}
	got := map[string]int{}
	for _, task := range m.Tasks {
		got[task.Name] = len(task.Lines)
	}
	for name, lineCount := range want {
		if got[name] != lineCount {
			t.Errorf("task %s line count = %d, want %d (lines: %v)",
				name, got[name], lineCount, findTask(m.Tasks, name).Lines)
		}
	}
}

func TestPreferencesValidation(t *testing.T) {
	m, err := Load(filepath.Join(examplesDir, "07-preferences.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if m.Preferences.Error == nil || *m.Preferences.Error != "Stop" {
		t.Errorf("preferences.error = %v, want Stop", m.Preferences.Error)
	}
	if m.Preferences.NativeErrorAction == nil || *m.Preferences.NativeErrorAction != true {
		t.Errorf("native-error-action = %v, want true", m.Preferences.NativeErrorAction)
	}
	if m.Preferences.MaxHistory == nil || *m.Preferences.MaxHistory != 4096 {
		t.Errorf("max-history = %v, want 4096", m.Preferences.MaxHistory)
	}
}

func TestPreferencesRejectsInvalidEnum(t *testing.T) {
	_, err := Parse([]byte(`
[project]
name = "x"

[powershell]
version = "7.5.6"

[preferences]
error = "Bogus"
`), "test")
	if err == nil {
		t.Fatal("expected error for invalid enum, got nil")
	}
}

func TestPreferencesRejectsOutOfRange(t *testing.T) {
	_, err := Parse([]byte(`
[project]
name = "x"

[powershell]
version = "7.5.6"

[preferences]
max-history = 100000
`), "test")
	if err == nil {
		t.Fatal("expected error for out-of-range max-history, got nil")
	}
}

func TestOutputModeRejectsUnknown(t *testing.T) {
	_, err := Parse([]byte(`
[project]
name = "x"

[powershell]
version = "7.5.6"

[output]
mode = "fancy"
`), "test")
	if err == nil {
		t.Fatal("expected error for unknown output mode")
	}
}

func TestUnknownKeyRejected(t *testing.T) {
	_, err := Parse([]byte(`
[project]
name = "x"
typo = "this should fail"

[powershell]
version = "7.5.6"
`), "test")
	if err == nil {
		t.Fatal("expected error for unknown key, got nil")
	}
}

func TestMissingRequired(t *testing.T) {
	_, err := Parse([]byte(`
[project]
name = "x"
`), "test")
	if err == nil {
		t.Fatal("expected error for missing powershell.version")
	}

	_, err = Parse([]byte(`
[powershell]
version = "7.5.6"
`), "test")
	if err == nil {
		t.Fatal("expected error for missing project.name")
	}
}

func findTask(ts []Task, name string) Task {
	for _, t := range ts {
		if t.Name == name {
			return t
		}
	}
	return Task{}
}
