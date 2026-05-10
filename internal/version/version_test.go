package version

import "testing"

func mustVersion(t *testing.T, s string) Version {
	t.Helper()
	v, err := ParseVersion(s)
	if err != nil {
		t.Fatalf("ParseVersion(%q): %v", s, err)
	}
	return v
}

func TestParseVersion(t *testing.T) {
	cases := []struct {
		in      string
		want    Version
		wantErr bool
	}{
		{"5.7.1", Version{Major: 5, Minor: 7, Patch: 1}, false},
		{"7.5.6", Version{Major: 7, Minor: 5, Patch: 6}, false},
		{"6.0.0-alpha1", Version{Major: 6, Minor: 0, Patch: 0, Prerelease: "alpha1"}, false},
		{"5.7.1-rc.1", Version{Major: 5, Minor: 7, Patch: 1, Prerelease: "rc.1"}, false},
		{"  7.5.6  ", Version{Major: 7, Minor: 5, Patch: 6}, false},

		{"5.7", Version{}, true},
		{"5.7.1.2", Version{}, true},
		{"v5.7.1", Version{}, true},
		{"-1.0.0", Version{}, true},
		{"", Version{}, true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParseVersion(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
			}
			if !tc.wantErr && got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.0", "1.0.1", -1},
		{"1.0.1", "1.0.0", +1},
		{"1.1.0", "1.0.99", +1},
		{"2.0.0", "1.99.99", +1},

		{"5.7.1-rc.1", "5.7.1", -1},
		{"5.7.1", "5.7.1-rc.1", +1},
		{"5.7.1-rc.1", "5.7.1-rc.2", -1},
		{"6.0.0-alpha1", "6.0.0-beta1", -1},
		{"6.0.0-alpha.1", "6.0.0-alpha.2", -1},
	}
	for _, tc := range cases {
		t.Run(tc.a+" vs "+tc.b, func(t *testing.T) {
			a := mustVersion(t, tc.a)
			b := mustVersion(t, tc.b)
			if got := a.Compare(b); got != tc.want {
				t.Fatalf("Compare(%s,%s) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestSpecParseAndMatch(t *testing.T) {
	type matchCase struct {
		ver  string
		want bool
	}
	cases := []struct {
		spec    string
		matches []matchCase
		wantErr bool
	}{
		{
			spec: "5.7.1",
			matches: []matchCase{
				{"5.7.1", true},
				{"5.7.0", false},
				{"5.7.2", false},
			},
		},
		{
			spec: "*",
			matches: []matchCase{
				{"1.0.0", true},
				{"99.99.99", true},
				{"1.0.0-rc.1", false}, // prereleases excluded
			},
		},
		{
			spec: "latest",
			matches: []matchCase{
				{"7.5.6", true},
				{"7.5.6-rc.1", false},
			},
		},
		{
			spec: "7.5.*",
			matches: []matchCase{
				{"7.5.0", true},
				{"7.5.99", true},
				{"7.6.0", false},
				{"7.4.99", false},
			},
		},
		{
			spec: ">=5.1.1,<6",
			matches: []matchCase{
				{"5.1.1", true},
				{"5.99.99", true},
				{"6.0.0", false},
				{"5.1.0", false},
			},
		},
		{
			spec: ">=2.35",
			matches: []matchCase{
				{"2.35.0", true},
				{"3.0.0", true},
				{"2.34.99", false},
			},
		},
		{
			spec: "5.7.1-rc.1",
			matches: []matchCase{
				{"5.7.1-rc.1", true},
				{"5.7.1", false},
			},
		},

		{spec: "not-a-version", wantErr: true},
		{spec: ">=", wantErr: true},
		{spec: "1.*", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.spec, func(t *testing.T) {
			s, err := ParseSpec(tc.spec)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
			}
			if tc.wantErr {
				return
			}
			for _, m := range tc.matches {
				v := mustVersion(t, m.ver)
				if got := s.Match(v); got != m.want {
					t.Errorf("Match(%s) = %v, want %v", m.ver, got, m.want)
				}
			}
		})
	}
}

func TestPick(t *testing.T) {
	candidates := []Version{
		mustVersion(t, "5.7.0"),
		mustVersion(t, "5.7.1"),
		mustVersion(t, "5.8.0"),
		mustVersion(t, "6.0.0-rc.1"),
	}

	pick := func(spec string) string {
		s, err := ParseSpec(spec)
		if err != nil {
			t.Fatalf("ParseSpec(%q): %v", spec, err)
		}
		v, ok := s.Pick(candidates)
		if !ok {
			return ""
		}
		return v.String()
	}

	if got := pick("*"); got != "5.8.0" {
		t.Errorf("Pick(*) = %q, want 5.8.0 (prereleases excluded)", got)
	}
	if got := pick("5.7.*"); got != "5.7.1" {
		t.Errorf("Pick(5.7.*) = %q, want 5.7.1", got)
	}
	if got := pick(">=5.7.0,<5.8.0"); got != "5.7.1" {
		t.Errorf("Pick range = %q, want 5.7.1", got)
	}
	if got := pick("6.0.0-rc.1"); got != "6.0.0-rc.1" {
		t.Errorf("Pick prerelease = %q, want 6.0.0-rc.1", got)
	}
	if got := pick("9.9.9"); got != "" {
		t.Errorf("Pick of nothing should be empty, got %q", got)
	}
}
