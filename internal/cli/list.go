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

func newListCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "Print a summary of the project's manifest",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			m, _, err := loadProject("")
			if err != nil {
				return err
			}
			return runList(m, format)
		},
	}
	cmd.Flags().StringVarP(&format, "format", "f", "",
		"output format: json, toml, yaml, xml (omit for human-readable)")
	return cmd
}

func runList(m *manifest.Manifest, format string) error {
	if format == "" {
		// Side-effect: prints the conch banner in TUI mode.
		_ = newUI(m.Output.Mode)
		printHumanList(m)
		return nil
	}

	view := manifestToView(m)
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

// listView is the common DTO every -f format encodes from. Field tags
// are duplicated for each codec because Go encoders don't share a
// vocabulary, but the layout is the same across all four formats.
//
// XML uses flat repetition (`<module>`, `<task>`, `<author>`,
// `<platform>` directly under their parent element) rather than the
// nested-wrapper form (`<modules><module>…`). encoding/xml's
// omitempty does not suppress nested wrappers when the inner slice
// is empty, so the wrapper would produce an empty `<modules></modules>`
// — the flat form sidesteps that without a custom MarshalXML.
type listView struct {
	XMLName    xml.Name     `json:"-" yaml:"-" toml:"-" xml:"conch"`
	Project    listProject  `json:"project" yaml:"project" toml:"project" xml:"project"`
	PowerShell listPwsh     `json:"powershell" yaml:"powershell" toml:"powershell" xml:"powershell"`
	Modules    []listModule `json:"modules,omitempty" yaml:"modules,omitempty" toml:"modules,omitempty" xml:"module,omitempty"`
	Tasks      []listTask   `json:"tasks,omitempty" yaml:"tasks,omitempty" toml:"tasks,omitempty" xml:"task,omitempty"`
	Output     *listOutCfg  `json:"output,omitempty" yaml:"output,omitempty" toml:"output,omitempty" xml:"output,omitempty"`
}

type listProject struct {
	Name        string   `json:"name" yaml:"name" toml:"name" xml:"name"`
	Version     string   `json:"version,omitempty" yaml:"version,omitempty" toml:"version,omitempty" xml:"version,omitempty"`
	Description string   `json:"description,omitempty" yaml:"description,omitempty" toml:"description,omitempty" xml:"description,omitempty"`
	Authors     []string `json:"authors,omitempty" yaml:"authors,omitempty" toml:"authors,omitempty" xml:"author,omitempty"`
	Platforms   []string `json:"platforms" yaml:"platforms" toml:"platforms" xml:"platform"`
}

type listPwsh struct {
	Version string `json:"version" yaml:"version" toml:"version" xml:"version"`
}

type listModule struct {
	Name string `json:"name" yaml:"name" toml:"name" xml:"name,attr"`
	Spec string `json:"spec" yaml:"spec" toml:"spec" xml:",chardata"`
}

type listTask struct {
	Name  string   `json:"name" yaml:"name" toml:"name" xml:"name,attr"`
	Lines []string `json:"lines" yaml:"lines" toml:"lines" xml:"line"`
}

type listOutCfg struct {
	Mode string `json:"mode,omitempty" yaml:"mode,omitempty" toml:"mode,omitempty" xml:"mode,omitempty"`
}

func manifestToView(m *manifest.Manifest) listView {
	v := listView{
		Project: listProject{
			Name:        m.Project.Name,
			Version:     m.Project.Version,
			Description: m.Project.Description,
			Authors:     m.Project.Authors,
			Platforms:   make([]string, len(m.Project.Platforms)),
		},
		PowerShell: listPwsh{Version: m.PowerShell.Version.Raw()},
	}
	for i, p := range m.Project.Platforms {
		v.Project.Platforms[i] = p.String()
	}
	for _, mod := range m.Modules {
		v.Modules = append(v.Modules, listModule{Name: mod.Name, Spec: mod.Spec.Raw()})
	}
	for _, t := range m.Tasks {
		v.Tasks = append(v.Tasks, listTask{Name: t.Name, Lines: t.Lines})
	}
	if m.Output.Mode != "" {
		v.Output = &listOutCfg{Mode: m.Output.Mode}
	}
	return v
}

func printHumanList(m *manifest.Manifest) {
	fmt.Println("project:")
	fmt.Printf("  name        %s\n", m.Project.Name)
	if m.Project.Version != "" {
		fmt.Printf("  version     %s\n", m.Project.Version)
	}
	if m.Project.Description != "" {
		fmt.Printf("  description %s\n", m.Project.Description)
	}
	if len(m.Project.Authors) > 0 {
		fmt.Printf("  authors     %s\n", strings.Join(m.Project.Authors, ", "))
	}
	fmt.Printf("  platforms   %s\n", formatPlatforms(m.Project.Platforms))
	fmt.Println()

	fmt.Printf("powershell: %s\n", m.PowerShell.Version.Raw())
	fmt.Println()

	if len(m.Modules) > 0 {
		fmt.Println("modules:")
		for _, mod := range m.Modules {
			fmt.Printf("  %-32s %s\n", mod.Name, mod.Spec.Raw())
		}
		fmt.Println()
	}

	if len(m.Tasks) > 0 {
		fmt.Println("tasks:")
		for _, t := range m.Tasks {
			first := ""
			if len(t.Lines) > 0 {
				first = t.Lines[0]
			}
			if len(t.Lines) > 1 {
				first += " …"
			}
			fmt.Printf("  %-12s %s\n", t.Name, first)
		}
	}
}
