package manifest

import (
	"fmt"
	"strings"
)

// Preferences mirrors the [preferences] table. Every field is a pointer:
// nil means "not set, use PowerShell's default", a non-nil value is
// emitted into the activation script as a `$PreferenceName = value`
// assignment.
//
// Field names are CamelCase; TOML keys use kebab-case. Refer to
// examples/07-preferences.toml for the canonical mapping.
type Preferences struct {
	// Action preferences — same allowed value set, different $variables.
	Error       *string `toml:"error"`
	Warning     *string `toml:"warning"`
	Verbose     *string `toml:"verbose"`
	Debug       *string `toml:"debug"`
	Information *string `toml:"information"`
	Progress    *string `toml:"progress"`

	Confirm          *string `toml:"confirm"`
	ErrorView        *string `toml:"error-view"`
	ModuleAutoload   *string `toml:"module-autoload"`
	NativeArgPassing *string `toml:"native-arg-passing"`

	WhatIf            *bool `toml:"whatif"`
	NativeErrorAction *bool `toml:"native-error-action"`

	FormatEnumLimit *int64 `toml:"format-enum-limit"`
	MaxHistory      *int64 `toml:"max-history"`

	OFS *string `toml:"ofs"`

	LogCommandHealth     *bool `toml:"log-command-health"`
	LogCommandLifecycle  *bool `toml:"log-command-lifecycle"`
	LogEngineHealth      *bool `toml:"log-engine-health"`
	LogEngineLifecycle   *bool `toml:"log-engine-lifecycle"`
	LogProviderHealth    *bool `toml:"log-provider-health"`
	LogProviderLifecycle *bool `toml:"log-provider-lifecycle"`
}

// Allowed value sets. Kept here rather than in init() so they're
// visible in source.
var (
	actionValues       = []string{"Continue", "SilentlyContinue", "Stop", "Inquire", "Ignore", "Break"}
	confirmValues      = []string{"None", "Low", "Medium", "High"}
	errorViewValues    = []string{"NormalView", "CategoryView", "ConciseView", "DetailedView"}
	moduleAutoloadVals = []string{"None", "ModuleQualified", "All"}
	nativeArgValues    = []string{"Legacy", "Standard", "Windows"}
)

func (p Preferences) validate() error {
	checks := []struct {
		key     string
		val     *string
		allowed []string
	}{
		{"error", p.Error, actionValues},
		{"warning", p.Warning, actionValues},
		{"verbose", p.Verbose, actionValues},
		{"debug", p.Debug, actionValues},
		{"information", p.Information, actionValues},
		{"progress", p.Progress, actionValues},
		{"confirm", p.Confirm, confirmValues},
		{"error-view", p.ErrorView, errorViewValues},
		{"module-autoload", p.ModuleAutoload, moduleAutoloadVals},
		{"native-arg-passing", p.NativeArgPassing, nativeArgValues},
	}
	for _, c := range checks {
		if c.val == nil {
			continue
		}
		if !contains(c.allowed, *c.val) {
			return fmt.Errorf(
				"[preferences].%s = %q: must be one of %s",
				c.key, *c.val, strings.Join(c.allowed, ", "),
			)
		}
	}

	if p.FormatEnumLimit != nil && *p.FormatEnumLimit < 0 {
		return fmt.Errorf("[preferences].format-enum-limit = %d: must be >= 0", *p.FormatEnumLimit)
	}
	if p.MaxHistory != nil && (*p.MaxHistory < 1 || *p.MaxHistory > 32768) {
		return fmt.Errorf("[preferences].max-history = %d: must be between 1 and 32768", *p.MaxHistory)
	}
	return nil
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}
