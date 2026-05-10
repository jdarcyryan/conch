package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/jdarcyryan/conch/internal/env"
	"github.com/jdarcyryan/conch/internal/platform"
)

// launchPwsh is the shared entry point for `conch run` and `conch shell`.
// It builds an exec.Cmd that:
//
//   - invokes the project's own .conch/pwsh/pwsh executable;
//   - prepends .conch/pwsh to PATH so that nested `pwsh` invocations,
//     `Get-Command pwsh`, and `which pwsh` from inside resolve to the
//     environment's binary instead of any host-installed pwsh;
//   - rewrites PSModulePath to point exclusively at .conch/modules;
//   - sets CONCH_ENV to the project root, useful for prompt customisation
//     and detecting nested activations.
//
// extraArgs are appended after the conch-supplied flags, so callers can
// add e.g. -NoExit (shell) or -Command "..." (run).
func launchPwsh(layout env.Layout, extraArgs []string) (*exec.Cmd, error) {
	host, err := platform.Current()
	if err != nil {
		return nil, err
	}
	exe := layout.PowerShellExecutable(host)
	if _, err := os.Stat(exe); os.IsNotExist(err) {
		return nil, fmt.Errorf("PowerShell not installed at %s — run `conch install` first", exe)
	}

	args := append([]string{"-NoLogo", "-NoProfile"}, extraArgs...)
	cmd := exec.Command(exe, args...)
	cmd.Env = withConchEnv(os.Environ(), layout)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd, nil
}

// withConchEnv overrides PSModulePath, prepends the env's pwsh dir to
// PATH, and exports CONCH_ENV. Any pre-existing values for those keys
// are dropped — the conch environment must shadow the host's, per
// CLAUDE.md.
func withConchEnv(base []string, layout env.Layout) []string {
	out := make([]string, 0, len(base)+3)
	originalPath := ""
	for _, kv := range base {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			out = append(out, kv)
			continue
		}
		switch strings.ToUpper(k) {
		case "PSMODULEPATH", "CONCH_ENV":
			// drop — we set our own below
			continue
		case "PATH":
			originalPath = v
			continue
		}
		out = append(out, kv)
	}

	pathParts := []string{layout.PowerShellDir()}
	if originalPath != "" {
		pathParts = append(pathParts, originalPath)
	}

	// PSModulePath: project modules first, then the modules bundled
	// with the PowerShell distribution itself. Without the bundled
	// pair, basic cmdlets like Add-Member become unavailable; see
	// env.Layout.BundledModulesDir.
	psModulePath := layout.ModulesDir() +
		string(os.PathListSeparator) +
		layout.BundledModulesDir()

	out = append(out,
		"PSModulePath="+psModulePath,
		"PATH="+strings.Join(pathParts, string(os.PathListSeparator)),
		"CONCH_ENV="+layout.ProjectRoot,
	)
	return out
}

// exitWith maps an exec.Cmd's run error onto an os.Exit code so a task
// or shell that exits non-zero propagates that to the conch caller.
func exitWith(err error) error {
	if err == nil {
		return nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		os.Exit(exitErr.ExitCode())
	}
	return err
}
