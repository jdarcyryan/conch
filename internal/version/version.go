// Package version implements the version and version-specifier grammar
// conch supports. It deliberately avoids a third-party semver library:
// the rules below are pixi-style and don't quite match either SemVer or
// NuGet ranges, so a small bespoke implementation is clearer than
// reshaping someone else's.
//
// Supported version syntax:
//
//	major.minor.patch                 e.g. 5.7.1
//	major.minor.patch-<prerelease>    e.g. 5.7.1-rc.1, 6.0.0-alpha1
//
// Supported specifier syntax:
//
//	"5.7.1"         exact version
//	"7.5.*"         wildcard within a minor line — any 7.5.x
//	">=2,<3"        range expression (comma-separated AND)
//	">=5.1.1"       lower bound only
//	"*"             any version, resolves to newest at install time
//	"latest"        synonym of "*"
//	"5.7.1-rc.1"    explicit prerelease — "*"/"latest" never resolve to
//	                prereleases.
package version

import (
	"fmt"
	"strconv"
	"strings"
)

// Version is a parsed conch version.
type Version struct {
	Major, Minor, Patch int
	Prerelease          string // empty for stable releases
}

// IsPrerelease reports whether this version carries a prerelease tag.
func (v Version) IsPrerelease() bool { return v.Prerelease != "" }

// String returns the canonical "major.minor.patch[-prerelease]" form.
func (v Version) String() string {
	out := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Prerelease != "" {
		out += "-" + v.Prerelease
	}
	return out
}

// Compare returns -1, 0, or +1 depending on whether v is older than,
// equal to, or newer than other. Prereleases sort before the
// corresponding release (5.7.1-rc.1 < 5.7.1), per SemVer.
func (v Version) Compare(other Version) int {
	if c := cmp(v.Major, other.Major); c != 0 {
		return c
	}
	if c := cmp(v.Minor, other.Minor); c != 0 {
		return c
	}
	if c := cmp(v.Patch, other.Patch); c != 0 {
		return c
	}
	switch {
	case v.Prerelease == "" && other.Prerelease == "":
		return 0
	case v.Prerelease == "":
		return +1
	case other.Prerelease == "":
		return -1
	}
	return comparePrerelease(v.Prerelease, other.Prerelease)
}

// ParseVersion decodes a "major.minor.patch[-prerelease]" string.
// Trailing whitespace is tolerated; missing components are not.
func ParseVersion(s string) (Version, error) {
	return parseVersion(s, false)
}

// parseVersion decodes a version string. When allowPartial is true,
// missing minor/patch components are zero-filled — useful inside range
// expressions where users routinely write `<6` to mean `<6.0.0`.
func parseVersion(s string, allowPartial bool) (Version, error) {
	raw := strings.TrimSpace(s)
	core, pre, _ := strings.Cut(raw, "-")
	parts := strings.Split(core, ".")
	switch {
	case len(parts) == 3:
		// fine
	case allowPartial && (len(parts) == 1 || len(parts) == 2):
		for len(parts) < 3 {
			parts = append(parts, "0")
		}
	default:
		return Version{}, fmt.Errorf("invalid version %q: expected major.minor.patch", s)
	}
	nums := make([]int, 3)
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return Version{}, fmt.Errorf("invalid version %q: %q is not a non-negative integer", s, p)
		}
		nums[i] = n
	}
	return Version{
		Major:      nums[0],
		Minor:      nums[1],
		Patch:      nums[2],
		Prerelease: pre,
	}, nil
}

// Spec is a parsed version specifier. It matches one or more concrete
// versions; Pick selects the newest matching candidate.
type Spec struct {
	raw         string
	any         bool         // "*" / "latest"
	allowPre    bool         // user wrote a prerelease component → allow them in matching
	constraints []constraint // ANDed together
}

type constraint struct {
	op  string // ">=", ">", "<=", "<", "="
	ver Version
}

// ParseSpec decodes a specifier string. An empty string is treated as
// "*" — most projects pin everything, but tasks-only manifests with no
// modules table should still parse cleanly.
func ParseSpec(s string) (Spec, error) {
	raw := strings.TrimSpace(s)
	if raw == "" || raw == "*" || strings.EqualFold(raw, "latest") {
		return Spec{raw: raw, any: true}, nil
	}

	// Wildcard within a minor line: "7.5.*" → ">=7.5.0,<7.6.0"
	if strings.HasSuffix(raw, ".*") {
		base := strings.TrimSuffix(raw, ".*")
		parts := strings.Split(base, ".")
		if len(parts) != 2 {
			return Spec{}, fmt.Errorf("invalid wildcard %q: expected major.minor.*", raw)
		}
		major, err := strconv.Atoi(parts[0])
		if err != nil || major < 0 {
			return Spec{}, fmt.Errorf("invalid wildcard %q: bad major", raw)
		}
		minor, err := strconv.Atoi(parts[1])
		if err != nil || minor < 0 {
			return Spec{}, fmt.Errorf("invalid wildcard %q: bad minor", raw)
		}
		lower := Version{Major: major, Minor: minor, Patch: 0}
		upper := Version{Major: major, Minor: minor + 1, Patch: 0}
		return Spec{
			raw: raw,
			constraints: []constraint{
				{op: ">=", ver: lower},
				{op: "<", ver: upper},
			},
		}, nil
	}

	// Range expression with explicit operators?
	if hasOp(raw) {
		return parseRange(raw)
	}

	// Otherwise it's an exact version.
	v, err := ParseVersion(raw)
	if err != nil {
		return Spec{}, err
	}
	return Spec{
		raw:         raw,
		allowPre:    v.IsPrerelease(),
		constraints: []constraint{{op: "=", ver: v}},
	}, nil
}

func parseRange(raw string) (Spec, error) {
	parts := strings.Split(raw, ",")
	out := Spec{raw: raw}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		op, rest := splitOp(p)
		if op == "" {
			return Spec{}, fmt.Errorf("invalid range %q: missing operator in %q", raw, p)
		}
		v, err := parseVersion(rest, true)
		if err != nil {
			return Spec{}, fmt.Errorf("invalid range %q: %w", raw, err)
		}
		if v.IsPrerelease() {
			out.allowPre = true
		}
		out.constraints = append(out.constraints, constraint{op: op, ver: v})
	}
	if len(out.constraints) == 0 {
		return Spec{}, fmt.Errorf("invalid range %q: no constraints", raw)
	}
	return out, nil
}

// IsAny reports whether the spec is "*" or "latest" — i.e. matches any
// stable release. Useful for resolution heuristics that cache
// "newest-stable" lookups.
func (s Spec) IsAny() bool { return s.any }

// Raw returns the original specifier string.
func (s Spec) Raw() string { return s.raw }

// Match reports whether v satisfies the specifier.
//
// Rules:
//   - "*" / "latest" matches any *stable* version (prereleases excluded).
//   - Other specs match according to their constraints; a candidate that
//     is itself a prerelease is rejected unless the spec explicitly
//     mentions a prerelease somewhere.
func (s Spec) Match(v Version) bool {
	if v.IsPrerelease() && !s.any && !s.allowPre {
		// Prereleases must be explicitly opted into.
		return false
	}
	if v.IsPrerelease() && s.any {
		return false
	}
	if s.any {
		return true
	}
	for _, c := range s.constraints {
		cmpResult := v.Compare(c.ver)
		switch c.op {
		case "=":
			if cmpResult != 0 {
				return false
			}
		case ">":
			if cmpResult <= 0 {
				return false
			}
		case ">=":
			if cmpResult < 0 {
				return false
			}
		case "<":
			if cmpResult >= 0 {
				return false
			}
		case "<=":
			if cmpResult > 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// Pick returns the newest version in candidates that satisfies the spec.
// Returns ok=false if nothing matched.
func (s Spec) Pick(candidates []Version) (Version, bool) {
	var best Version
	found := false
	for _, c := range candidates {
		if !s.Match(c) {
			continue
		}
		if !found || c.Compare(best) > 0 {
			best = c
			found = true
		}
	}
	return best, found
}

// --- helpers ----------------------------------------------------------------

func hasOp(s string) bool {
	for _, op := range []string{">=", "<=", ">", "<"} {
		if strings.Contains(s, op) {
			return true
		}
	}
	return false
}

func splitOp(s string) (op, rest string) {
	for _, candidate := range []string{">=", "<=", ">", "<", "="} {
		if strings.HasPrefix(s, candidate) {
			return candidate, strings.TrimSpace(strings.TrimPrefix(s, candidate))
		}
	}
	return "", s
}

func cmp(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return +1
	}
	return 0
}

// comparePrerelease compares two non-empty prerelease tags following
// SemVer 2.0.0: identifiers split on '.', numeric vs alphanumeric
// ordering, and shorter tag with all matching prefix sorts lower.
func comparePrerelease(a, b string) int {
	ai := strings.Split(a, ".")
	bi := strings.Split(b, ".")
	n := len(ai)
	if len(bi) < n {
		n = len(bi)
	}
	for i := 0; i < n; i++ {
		if c := compareIdent(ai[i], bi[i]); c != 0 {
			return c
		}
	}
	return cmp(len(ai), len(bi))
}

func compareIdent(a, b string) int {
	an, aErr := strconv.Atoi(a)
	bn, bErr := strconv.Atoi(b)
	switch {
	case aErr == nil && bErr == nil:
		return cmp(an, bn)
	case aErr == nil:
		return -1 // numeric < alphanumeric
	case bErr == nil:
		return +1
	}
	return strings.Compare(a, b)
}
