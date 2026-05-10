package cli

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/jdarcyryan/conch/internal/env"
	"github.com/jdarcyryan/conch/internal/lockfile"
	"github.com/jdarcyryan/conch/internal/manifest"
	"github.com/jdarcyryan/conch/internal/platform"
	"github.com/jdarcyryan/conch/internal/version"
)

// isInstalled reports whether the project at layout has a fully
// materialised environment matching m on host. The check is local
// only — no network — so it's safe to run unconditionally before any
// command that might otherwise trigger an install.
//
// "Fully materialised" means:
//
//   - the PowerShell executable for host exists,
//   - the lockfile records a PowerShell entry for host whose version
//     still satisfies the manifest's [powershell].version spec,
//   - the lockfile records every manifest module, the recorded
//     version still satisfies that module's spec, and the
//     <Name>/<Version>/<Name>.psd1 file is present on disk.
//
// Note: activate.ps1 is *not* part of this check. Tasks and
// preferences live only in that script, and the user can edit them
// without changing any artefact this function sees, so callers
// regenerate it unconditionally — see ensureInstalled.
func isInstalled(layout env.Layout, m *manifest.Manifest, lock *lockfile.Lockfile, host platform.Platform) bool {
	if !fileExists(layout.PowerShellExecutable(host)) {
		return false
	}
	if lock == nil {
		return false
	}
	pwshLock, ok := lock.LookupPowerShell(host.String())
	if !ok {
		return false
	}
	if !versionSatisfies(pwshLock.Version, m.PowerShell.Version) {
		return false
	}
	for _, mod := range m.Modules {
		entry, ok := lock.LookupModule(mod.Name)
		if !ok {
			return false
		}
		if !versionSatisfies(entry.Version, mod.Spec) {
			return false
		}
		manifestPath := filepath.Join(layout.ModulesDir(), mod.Name, entry.Version, mod.Name+".psd1")
		if !fileExists(manifestPath) {
			return false
		}
	}
	return true
}

// versionSatisfies parses a lockfile-recorded version string and
// reports whether it still matches the manifest's spec. An
// unparseable version (which would be a bug elsewhere) counts as a
// miss so we re-resolve and overwrite it.
func versionSatisfies(recorded string, spec version.Spec) bool {
	v, err := version.ParseVersion(recorded)
	if err != nil {
		return false
	}
	return spec.Match(v)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// readOptionalLockfile loads conch.lock if present. A missing file
// returns (nil, nil) — that's the normal "fresh project" path.
func readOptionalLockfile(path string) (*lockfile.Lockfile, error) {
	l, err := lockfile.Read(path)
	if err == nil {
		return l, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return nil, err
}
