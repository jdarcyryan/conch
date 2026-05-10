package platform

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		in      string
		want    Platform
		wantErr bool
	}{
		{"windows-amd64", Platform{Windows, AMD64}, false},
		{"windows-arm64", Platform{Windows, ARM64}, false},
		{"linux-amd64", Platform{Linux, AMD64}, false},
		{"linux-arm64", Platform{Linux, ARM64}, false},

		{"darwin-amd64", Platform{}, true},
		{"windows-386", Platform{}, true},
		{"windows", Platform{}, true},
		{"", Platform{}, true},
		{"linux-amd64-extra", Platform{}, true},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got, err := Parse(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
			}
			if !tc.wantErr && got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestPowerShellNaming(t *testing.T) {
	cases := []struct {
		p           Platform
		wantOS      string
		wantArch    string
		wantExt     string
		wantArchive string
	}{
		{Platform{Windows, AMD64}, "win", "x64", ".zip", "windows-amd64"},
		{Platform{Windows, ARM64}, "win", "arm64", ".zip", "windows-arm64"},
		{Platform{Linux, AMD64}, "linux", "x64", ".tar.gz", "linux-amd64"},
		{Platform{Linux, ARM64}, "linux", "arm64", ".tar.gz", "linux-arm64"},
	}
	for _, tc := range cases {
		t.Run(tc.p.String(), func(t *testing.T) {
			if got := tc.p.PowerShellOS(); got != tc.wantOS {
				t.Errorf("PowerShellOS = %q, want %q", got, tc.wantOS)
			}
			if got := tc.p.PowerShellArch(); got != tc.wantArch {
				t.Errorf("PowerShellArch = %q, want %q", got, tc.wantArch)
			}
			if got := tc.p.PowerShellExt(); got != tc.wantExt {
				t.Errorf("PowerShellExt = %q, want %q", got, tc.wantExt)
			}
			if got := tc.p.String(); got != tc.wantArchive {
				t.Errorf("String = %q, want %q", got, tc.wantArchive)
			}
		})
	}
}

func TestAllSupported(t *testing.T) {
	for _, p := range All() {
		if !p.Supported() {
			t.Errorf("%v should be supported", p)
		}
	}
}
