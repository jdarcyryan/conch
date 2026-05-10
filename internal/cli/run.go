package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jdarcyryan/conch/internal/manifest"
	"github.com/jdarcyryan/conch/internal/platform"
)

func newRunCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run TASK [args...]",
		Short: "Run a task defined in conch.toml inside the project's environment",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTask(args[0], args[1:])
		},
	}
	// Pass-through to the auto-install step that fires when the
	// environment is missing — once pwsh takes over, conch isn't
	// rendering anything anyway, so the flag only meaningfully
	// affects install output.
	cmd.Flags().BoolVarP(&minUI, "min-ui", "m", false,
		"plain timestamped log output for any auto-install step")
	return cmd
}

func runTask(name string, extraArgs []string) error {
	m, layout, err := loadProject("")
	if err != nil {
		return err
	}

	taskLines, ok := lookupTaskLines(m, name)
	if !ok {
		return fmt.Errorf("no task named %q", name)
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

	u := newUI()
	if err := ensureInstalled(u, m, layout, host, true); err != nil {
		return err
	}

	psArgs := []string{"-Command", layout.TaskScript(taskLines)}
	psArgs = append(psArgs, extraArgs...)

	cmd, err := launchPwsh(layout, psArgs)
	if err != nil {
		return err
	}
	return exitWith(cmd.Run())
}

// lookupTaskLines returns the body of the named task as a slice of
// script lines. Task bodies live exclusively in conch.toml; this is
// re-read on every invocation so edits take effect immediately.
func lookupTaskLines(m *manifest.Manifest, name string) ([]string, bool) {
	for _, t := range m.Tasks {
		if t.Name == name {
			return t.Lines, true
		}
	}
	return nil, false
}
