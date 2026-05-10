package manifest

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/jdarcyryan/conch/internal/platform"
	"github.com/jdarcyryan/conch/internal/version"
)

// rawManifest mirrors conch.toml's wire format before any conversion.
// Keep this layout in lock-step with examples/.
type rawManifest struct {
	Project     rawProject         `toml:"project"`
	PowerShell  rawPowerShell      `toml:"powershell"`
	Modules     map[string]string  `toml:"modules"`
	Tasks       map[string]rawTask `toml:"tasks"`
	Preferences Preferences        `toml:"preferences"`
}

type rawProject struct {
	Name        string   `toml:"name"`
	Version     string   `toml:"version"`
	Description string   `toml:"description"`
	Authors     []string `toml:"authors"`
	Platforms   []string `toml:"platforms"`
}

type rawPowerShell struct {
	Version string `toml:"version"`
}

// rawTask is a string | []string variant. BurntSushi calls UnmarshalTOML
// with the raw decoded value (string or []interface{} for arrays).
type rawTask struct {
	lines []string
}

// UnmarshalTOML implements BurntSushi/toml.Unmarshaler.
func (t *rawTask) UnmarshalTOML(v any) error {
	switch x := v.(type) {
	case string:
		// Multi-line basic strings carry literal newlines; split into
		// lines so single-line and multi-line forms collapse to the
		// same downstream representation. Normalise CRLF first — TOML
		// files authored on Windows commonly arrive with \r\n, and
		// without this every script line would carry a trailing \r
		// that pollutes JSON/YAML output and the runtime task
		// invocation. Trim a single trailing newline (TOML """..."""
		// preserves it) so we don't end up with a spurious empty
		// line at the end of the script.
		s := strings.ReplaceAll(x, "\r\n", "\n")
		s = strings.TrimSuffix(s, "\n")
		if s == "" {
			t.lines = []string{""}
			return nil
		}
		t.lines = strings.Split(s, "\n")
		return nil
	case []any:
		for i, item := range x {
			s, ok := item.(string)
			if !ok {
				return fmt.Errorf("element %d is %T, expected string", i, item)
			}
			t.lines = append(t.lines, strings.TrimRight(s, "\r"))
		}
		return nil
	default:
		return fmt.Errorf("expected string or array of strings, got %T", v)
	}
}

// Load reads and parses a conch.toml file from disk.
func Load(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return Parse(data, path)
}

// Parse decodes a conch.toml document. The path argument is only used
// for error messages.
func Parse(data []byte, path string) (*Manifest, error) {
	var raw rawManifest
	meta, err := toml.Decode(string(data), &raw)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		keys := make([]string, len(undecoded))
		for i, k := range undecoded {
			keys[i] = k.String()
		}
		return nil, fmt.Errorf(
			"parse %s: unknown key(s): %s",
			path, strings.Join(keys, ", "),
		)
	}

	m, err := convert(raw)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := m.validate(); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := m.Preferences.validate(); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return m, nil
}

// convert turns the wire-shape rawManifest into a typed Manifest. It
// performs every conversion that needs failure handling — version spec
// parsing, platform parsing — and bubbles errors out with field
// context so the user sees exactly which entry was wrong.
func convert(raw rawManifest) (*Manifest, error) {
	m := &Manifest{
		Project: Project{
			Name:        raw.Project.Name,
			Version:     raw.Project.Version,
			Description: raw.Project.Description,
			Authors:     raw.Project.Authors,
		},
		Preferences: raw.Preferences,
	}

	for _, s := range raw.Project.Platforms {
		p, err := platform.Parse(s)
		if err != nil {
			return nil, fmt.Errorf("[project].platforms: %w", err)
		}
		m.Project.Platforms = append(m.Project.Platforms, p)
	}

	if raw.PowerShell.Version != "" {
		spec, err := version.ParseSpec(raw.PowerShell.Version)
		if err != nil {
			return nil, fmt.Errorf("[powershell].version: %w", err)
		}
		m.PowerShell.Version = spec
	}

	for name, spec := range raw.Modules {
		s, err := version.ParseSpec(spec)
		if err != nil {
			return nil, fmt.Errorf("[modules].%q: %w", name, err)
		}
		m.Modules = append(m.Modules, Module{Name: name, Spec: s})
	}
	sort.Slice(m.Modules, func(i, j int) bool { return m.Modules[i].Name < m.Modules[j].Name })

	for name, t := range raw.Tasks {
		m.Tasks = append(m.Tasks, Task{Name: name, Lines: t.lines})
	}
	sort.Slice(m.Tasks, func(i, j int) bool { return m.Tasks[i].Name < m.Tasks[j].Name })

	return m, nil
}
