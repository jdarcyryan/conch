package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/jdarcyryan/conch/internal/env"
	"github.com/jdarcyryan/conch/internal/platform"
)

func newShellCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shell",
		Short: "Launch an interactive PowerShell session inside the project's environment",
		Long: `Launch the project's own PowerShell with the conch environment active.

The launched session has:

  * .conch/pwsh prepended to PATH, so nested 'pwsh' invocations and
    'Get-Command pwsh' resolve to the environment's binary.
  * PSModulePath rewritten to .conch/modules + the bundled
    PowerShell modules.
  * activate.ps1 sourced — preferences applied, task functions defined.

If anything is missing, the environment is installed automatically
before the shell starts.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
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

			u := newUI()
			if err := ensureInstalled(u, m, layout, host, true); err != nil {
				return err
			}

			activate := fmt.Sprintf(". %s", env.PSSingleQuote(layout.ActivateScript()))
			psArgs := []string{"-NoExit", "-Command", activate}

			c, err := launchPwsh(layout, psArgs)
			if err != nil {
				return err
			}
			return exitWith(c.Run())
		},
	}
	// Pass-through to the auto-install step that fires when the
	// environment is missing. Once pwsh takes over the terminal,
	// conch isn't rendering anything anyway, so this flag only
	// meaningfully affects install output.
	cmd.Flags().BoolVarP(&minUI, "min-ui", "m", false,
		"plain timestamped log output for any auto-install step")
	return cmd
}
