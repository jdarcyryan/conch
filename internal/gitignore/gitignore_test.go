package gitignore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureCreatesIfMissing(t *testing.T) {
	dir := t.TempDir()
	changed, err := EnsureConchIgnored(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Errorf("expected changed=true on first call")
	}
	body, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if !strings.Contains(string(body), ".conch/") {
		t.Errorf("missing .conch/ in gitignore: %q", body)
	}
}

func TestEnsureIdempotent(t *testing.T) {
	dir := t.TempDir()
	if _, err := EnsureConchIgnored(dir); err != nil {
		t.Fatal(err)
	}
	changed, err := EnsureConchIgnored(dir)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Errorf("expected changed=false on second call")
	}
}

func TestEnsurePreservesExisting(t *testing.T) {
	dir := t.TempDir()
	existing := "node_modules/\n*.log\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := EnsureConchIgnored(dir); err != nil {
		t.Fatal(err)
	}
	body, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	got := string(body)
	if !strings.Contains(got, "node_modules/") {
		t.Errorf("lost node_modules/ entry: %q", got)
	}
	if !strings.Contains(got, "*.log") {
		t.Errorf("lost *.log entry: %q", got)
	}
	if !strings.Contains(got, ".conch/") {
		t.Errorf("missing .conch/ entry: %q", got)
	}
}

func TestEnsureRecognisesEquivalentForms(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("/.conch/\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	changed, err := EnsureConchIgnored(dir)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Errorf("expected changed=false for equivalent /.conch/ entry")
	}
}
