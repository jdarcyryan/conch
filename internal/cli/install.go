package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/jdarcyryan/conch/internal/cache"
	"github.com/jdarcyryan/conch/internal/env"
	"github.com/jdarcyryan/conch/internal/gitignore"
	"github.com/jdarcyryan/conch/internal/lockfile"
	"github.com/jdarcyryan/conch/internal/manifest"
	"github.com/jdarcyryan/conch/internal/platform"
	"github.com/jdarcyryan/conch/internal/psgallery"
	"github.com/jdarcyryan/conch/internal/pwsh"
	"github.com/jdarcyryan/conch/internal/ui"
)

func newInstallCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Resolve, download, and extract everything declared in conch.toml",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstall()
		},
	}
	cmd.Flags().BoolVarP(&minUI, "min-ui", "m", false,
		"plain timestamped log output instead of the colourful TUI")
	return cmd
}

// runInstall is the install command entrypoint. It always builds a
// fresh UI and prints the banner; idempotent short-circuit happens
// inside ensureInstalled.
func runInstall() error {
	m, layout, err := loadProject("")
	if err != nil {
		return err
	}

	host, err := platform.Current()
	if err != nil {
		return err
	}
	if !m.SupportsPlatform(host) {
		return fmt.Errorf(
			"this project does not support %s — manifest declares: %s",
			host, formatPlatforms(m.Project.Platforms),
		)
	}

	u := newUI(m.Output.Mode)
	return ensureInstalled(u, m, layout, host, true)
}

// ensureInstalled materialises the project's environment if anything
// is missing, otherwise reports "already up to date".
//
// The verbose flag controls whether a "ready" line is printed when
// nothing was missing. install passes true (the user just asked for
// it); shell/run pass true too so users see why their command was
// fast.
//
// activate.ps1 is regenerated on every call because tasks and
// preferences are not tracked anywhere else — editing conch.toml's
// [tasks] table without changing module versions would otherwise
// leave the old activate.ps1 in place, and `conch run new-task`
// would fail looking for a function that no script defines.
func ensureInstalled(u *ui.UI, m *manifest.Manifest, layout env.Layout, host platform.Platform, verbose bool) error {
	lock, err := readOptionalLockfile(layout.LockfilePath())
	if err != nil {
		return fmt.Errorf("read lockfile: %w", err)
	}
	if isInstalled(layout, m, lock, host) {
		if err := layout.WriteActivate(m); err != nil {
			return fmt.Errorf("regenerate activate.ps1: %w", err)
		}
		if verbose {
			u.Done("environment already up to date")
		}
		return nil
	}
	if lock == nil {
		lock = lockfile.New()
	}

	c, err := cache.Default()
	if err != nil {
		return err
	}

	u.Step("resolving PowerShell %s for %s", m.PowerShell.Version.Raw(), host)
	resolver := pwsh.Resolver{}
	rel, err := resolver.Resolve(m.PowerShell.Version)
	if err != nil {
		return fmt.Errorf("resolve PowerShell: %w", err)
	}
	u.Done("PowerShell %s", rel.Version)

	// If the lockfile already pinned this exact PowerShell version,
	// pass its SHA-256 down so the cache can short-circuit instead of
	// re-downloading. Versions don't match → discard the stale hash;
	// trusting it would be trusting a different artefact.
	pwExpectedSHA := ""
	if prev, ok := lock.LookupPowerShell(host.String()); ok && prev.Version == rel.Version.String() {
		pwExpectedSHA = prev.SHA256
	}

	pwInst := pwsh.Installer{Cache: c, UI: u}
	u.Step("installing PowerShell into %s", layout.PowerShellDir())
	pwSHA, err := pwInst.Install(rel, host, layout.PowerShellDir(), pwExpectedSHA)
	if err != nil {
		return fmt.Errorf("install PowerShell: %w", err)
	}
	lock.PutPowerShell(host.String(), lockfile.ResolvedItem{
		Version: rel.Version.String(),
		URL:     pwsh.DownloadURL(rel, pwsh.ArtefactName(rel, host)),
		SHA256:  pwSHA,
	})

	if err := os.MkdirAll(layout.ModulesDir(), 0o755); err != nil {
		return fmt.Errorf("create modules dir: %w", err)
	}

	psgClient := &psgallery.Client{}
	psgInst := psgallery.Installer{Cache: c, UI: u}
	for _, mod := range m.Modules {
		u.Step("resolving module %s %s", mod.Name, mod.Spec.Raw())
		pkg, err := psgClient.Resolve(mod.Name, mod.Spec)
		if err != nil {
			return fmt.Errorf("resolve %s: %w", mod.Name, err)
		}
		u.Done("%s %s", pkg.Name, pkg.Version)

		// Same lockfile-as-trust-anchor pattern as PowerShell. Same
		// version + matching SHA → cache hit, no re-download. Stale
		// version pin → discard. Modules are not per-platform in the
		// lockfile; PSGallery serves the same .nupkg everywhere.
		modExpectedSHA := ""
		if prev, ok := lock.LookupModule(pkg.Name); ok && prev.Version == pkg.Version.String() {
			modExpectedSHA = prev.SHA256
		}

		modSHA, err := psgInst.Install(pkg, layout.ModulesDir(), modExpectedSHA)
		if err != nil {
			return fmt.Errorf("install %s: %w", mod.Name, err)
		}
		lock.PutModule(pkg.Name, lockfile.ResolvedItem{
			Version: pkg.Version.String(),
			URL:     pkg.URL,
			SHA256:  modSHA,
		})
	}

	u.Step("writing activate.ps1")
	if err := layout.WriteActivate(m); err != nil {
		return err
	}
	u.Done("activate.ps1 written")

	u.Step("writing conch.lock")
	if err := lock.Write(layout.LockfilePath()); err != nil {
		return err
	}
	u.Done("conch.lock written")

	if changed, err := gitignore.EnsureConchIgnored(layout.ProjectRoot); err != nil {
		return err
	} else if changed {
		u.Done("added .conch/ to .gitignore")
	}

	u.Done("environment ready — `conch shell` to enter, `conch run TASK` to run a task")
	return nil
}

func formatPlatforms(ps []platform.Platform) string {
	parts := make([]string, len(ps))
	for i, p := range ps {
		parts[i] = p.String()
	}
	return strings.Join(parts, ", ")
}
