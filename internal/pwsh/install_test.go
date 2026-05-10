package pwsh

import (
	"testing"

	"github.com/jdarcyryan/conch/internal/platform"
	"github.com/jdarcyryan/conch/internal/version"
)

func TestArtefactName(t *testing.T) {
	rel := Release{
		Tag:     "v7.5.6",
		Version: version.Version{Major: 7, Minor: 5, Patch: 6},
	}
	cases := []struct {
		platform platform.Platform
		want     string
	}{
		// Windows: PascalCase prefix + .zip
		{platform.Platform{OS: platform.Windows, Arch: platform.AMD64}, "PowerShell-7.5.6-win-x64.zip"},
		{platform.Platform{OS: platform.Windows, Arch: platform.ARM64}, "PowerShell-7.5.6-win-arm64.zip"},
		// Linux: lowercase prefix + .tar.gz
		{platform.Platform{OS: platform.Linux, Arch: platform.AMD64}, "powershell-7.5.6-linux-x64.tar.gz"},
		{platform.Platform{OS: platform.Linux, Arch: platform.ARM64}, "powershell-7.5.6-linux-arm64.tar.gz"},
	}
	for _, tc := range cases {
		t.Run(tc.platform.String(), func(t *testing.T) {
			if got := ArtefactName(rel, tc.platform); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
