package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jdarcyryan/conch/internal/env"
	"github.com/jdarcyryan/conch/internal/manifest"
)

// loadProject locates conch.toml at projectRoot (which falls back to
// the current directory when empty), parses it, and returns the
// manifest along with a Layout rooted at the same directory.
func loadProject(projectRoot string) (*manifest.Manifest, env.Layout, error) {
	root, err := resolveRoot(projectRoot)
	if err != nil {
		return nil, env.Layout{}, err
	}
	layout := env.New(root)
	m, err := manifest.Load(layout.ManifestPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, layout, fmt.Errorf("no conch.toml found at %s — run `conch init` first", layout.ManifestPath())
		}
		return nil, layout, err
	}
	return m, layout, nil
}

func resolveRoot(projectRoot string) (string, error) {
	if projectRoot == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("locate working directory: %w", err)
		}
		return cwd, nil
	}
	abs, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", fmt.Errorf("resolve project root %q: %w", projectRoot, err)
	}
	return abs, nil
}
