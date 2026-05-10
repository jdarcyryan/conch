package lockfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "conch.lock")

	l := New()
	l.PutPowerShell("windows-amd64", ResolvedItem{
		Version: "7.5.6",
		URL:     "https://example.com/PowerShell-7.5.6-win-x64.zip",
		SHA256:  "abc",
	})
	l.PutPowerShell("linux-amd64", ResolvedItem{
		Version: "7.5.6",
		URL:     "https://example.com/PowerShell-7.5.6-linux-x64.tar.gz",
		SHA256:  "ghi",
	})
	l.PutModule("Pester", ResolvedItem{
		Version: "5.7.1",
		URL:     "https://example.com/pester.5.7.1.nupkg",
		SHA256:  "def",
	})

	if err := l.Write(path); err != nil {
		t.Fatal(err)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != SchemaVersion {
		t.Errorf("version = %d, want %d", got.Version, SchemaVersion)
	}
	if got.PowerShell["windows-amd64"].SHA256 != "abc" {
		t.Errorf("windows-amd64 sha mismatch: %v", got.PowerShell["windows-amd64"])
	}
	if got.Modules["Pester"].Version != "5.7.1" {
		t.Errorf("Pester version mismatch: %v", got.Modules["Pester"])
	}
}

func TestModulesAreNotPerPlatform(t *testing.T) {
	// One module entry serves every platform — PSGallery returns the
	// same .nupkg regardless of host OS, so the lockfile must not
	// duplicate per-platform.
	l := New()
	l.PutModule("Pester", ResolvedItem{Version: "5.7.1", SHA256: "x"})

	if got, ok := l.LookupModule("Pester"); !ok || got.Version != "5.7.1" {
		t.Errorf("expected Pester 5.7.1, got %v ok=%v", got, ok)
	}
}

func TestPutReplaces(t *testing.T) {
	l := New()
	l.PutModule("Pester", ResolvedItem{Version: "5.7.0"})
	l.PutModule("Pester", ResolvedItem{Version: "5.7.1"})

	if got, _ := l.LookupModule("Pester"); got.Version != "5.7.1" {
		t.Errorf("expected replace to win, got %v", got)
	}
	if len(l.Modules) != 1 {
		t.Errorf("expected 1 module entry, got %d", len(l.Modules))
	}
}

func TestRefusesNewerSchema(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "conch.lock")

	if err := os.WriteFile(path, []byte("version = 99\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(path); err == nil {
		t.Fatal("expected Read to reject newer schema, got nil")
	}
}

func TestNoArrayOfTablesInOutput(t *testing.T) {
	// Aesthetic regression: the on-disk file should not use TOML's
	// `[[entry]]` array-of-tables syntax. Keyed sub-tables are
	// easier to scan and diff.
	tmp := t.TempDir()
	path := filepath.Join(tmp, "conch.lock")

	l := New()
	l.PutPowerShell("windows-amd64", ResolvedItem{Version: "7.5.6"})
	l.PutModule("Pester", ResolvedItem{Version: "5.7.1"})

	if err := l.Write(path); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(path)
	if strings.Contains(string(body), "[[") {
		t.Errorf("lockfile should not contain '[[' sections:\n%s", body)
	}
}
