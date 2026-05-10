// Package gitignore handles the small but useful chore of making sure
// the project's .gitignore lists `.conch/` once an environment has
// been materialised. Idempotent — running it twice does nothing the
// second time.
package gitignore

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnsureConchIgnored adds a `.conch/` line to projectRoot/.gitignore
// if it isn't already there. Reports whether the file was modified.
//
// "Already there" is checked line-by-line against the canonical forms
// (`.conch`, `.conch/`, `/.conch/`); a glob like `.conch*` is *not*
// treated as a hit because it might be doing something different.
func EnsureConchIgnored(projectRoot string) (changed bool, err error) {
	path := filepath.Join(projectRoot, ".gitignore")
	existing, err := readGitignore(path)
	if err != nil {
		return false, err
	}
	if isAlreadyIgnored(existing) {
		return false, nil
	}

	// Append a blank line if the file is non-empty and doesn't already
	// end with one — mirrors how a developer would add a section by
	// hand.
	var b strings.Builder
	b.WriteString(existing)
	if existing != "" && !strings.HasSuffix(existing, "\n") {
		b.WriteString("\n")
	}
	if existing != "" && !strings.HasSuffix(existing, "\n\n") {
		b.WriteString("\n")
	}
	b.WriteString("# conch\n")
	b.WriteString(".conch/\n")

	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		return false, fmt.Errorf("write %s: %w", path, err)
	}
	return true, nil
}

func readGitignore(path string) (string, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return string(data), nil
}

func isAlreadyIgnored(content string) bool {
	canonical := map[string]struct{}{
		".conch":   {},
		".conch/":  {},
		"/.conch":  {},
		"/.conch/": {},
	}
	sc := bufio.NewScanner(strings.NewReader(content))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if _, ok := canonical[line]; ok {
			return true
		}
	}
	return false
}
