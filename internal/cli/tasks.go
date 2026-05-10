package cli

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"os"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"
	yaml "gopkg.in/yaml.v3"

	"github.com/jdarcyryan/conch/internal/manifest"
)

func newTasksCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "tasks",
		Short: "Show every task defined in conch.toml with its full body",
		Long: `Print every entry from [tasks] in conch.toml with its full body.

Single-line, multi-line, and array task forms all collapse to a list
of script lines so the body is rendered uniformly. Use --format to
emit machine-readable JSON, YAML, TOML, or XML instead.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, _, err := loadProject("")
			if err != nil {
				return err
			}
			return runTasks(m, format)
		},
	}
	cmd.Flags().StringVarP(&format, "format", "f", "",
		"output format: json, toml, yaml, xml (omit for human-readable)")
	return cmd
}

func runTasks(m *manifest.Manifest, format string) error {
	if format == "" {
		// Side-effect: prints the conch banner in TUI mode.
		_ = newUI(m.Output.Mode)
		printHumanTasks(m)
		return nil
	}

	view := tasksToView(m)
	switch strings.ToLower(format) {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(view)
	case "yaml", "yml":
		enc := yaml.NewEncoder(os.Stdout)
		enc.SetIndent(2)
		defer enc.Close()
		return enc.Encode(view)
	case "toml":
		return toml.NewEncoder(os.Stdout).Encode(view)
	case "xml":
		fmt.Fprint(os.Stdout, xml.Header)
		enc := xml.NewEncoder(os.Stdout)
		enc.Indent("", "  ")
		if err := enc.Encode(view); err != nil {
			return err
		}
		fmt.Fprintln(os.Stdout)
		return nil
	}
	return fmt.Errorf("unknown format %q: must be one of json, yaml, toml, xml", format)
}

// tasksView is the encoder DTO for `conch tasks --format`. The XML
// shape uses flat `<task>` repetition for the same omitempty reason
// listView documents.
type tasksView struct {
	XMLName xml.Name   `json:"-" yaml:"-" toml:"-" xml:"tasks"`
	Tasks   []listTask `json:"tasks" yaml:"tasks" toml:"tasks" xml:"task"`
}

func tasksToView(m *manifest.Manifest) tasksView {
	v := tasksView{Tasks: make([]listTask, 0, len(m.Tasks))}
	for _, t := range m.Tasks {
		v.Tasks = append(v.Tasks, listTask{Name: t.Name, Lines: t.Lines})
	}
	return v
}

func printHumanTasks(m *manifest.Manifest) {
	if len(m.Tasks) == 0 {
		fmt.Println("(no tasks defined)")
		return
	}
	for i, t := range m.Tasks {
		if i > 0 {
			fmt.Println()
		}
		fmt.Printf("%s:\n", t.Name)
		for _, line := range t.Lines {
			fmt.Printf("  %s\n", line)
		}
	}
}
