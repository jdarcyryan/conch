// Package platform deals with the OS/arch matrix conch supports and the
// translation between Go's runtime naming and PowerShell's release
// artefact naming.
//
// The supported matrix is fixed by goreleaser.yaml:
//
//	windows-amd64, windows-arm64, linux-amd64, linux-arm64
//
// PowerShell's official releases use a different naming scheme — `win`
// instead of `windows`, `x64` instead of `amd64`. The PowerShell* helpers
// translate to that scheme; the Go-style strings ("windows-amd64") are
// what conch uses everywhere else, including in conch.toml.
package platform

import (
	"fmt"
	"runtime"
	"strings"
)

// OS is the operating system half of a platform pair.
type OS string

const (
	Windows OS = "windows"
	Linux   OS = "linux"
)

// Arch is the architecture half of a platform pair.
type Arch string

const (
	AMD64 Arch = "amd64"
	ARM64 Arch = "arm64"
)

// Platform is one OS/arch combination conch supports.
type Platform struct {
	OS   OS
	Arch Arch
}

// All returns every OS/arch combination conch supports, in a stable order.
// Used as the default platform set when a manifest doesn't specify one.
func All() []Platform {
	return []Platform{
		{Windows, AMD64},
		{Windows, ARM64},
		{Linux, AMD64},
		{Linux, ARM64},
	}
}

// String returns the canonical "<os>-<arch>" form, e.g. "windows-amd64".
// This is the form used in conch.toml's `platforms` field and in the
// lockfile.
func (p Platform) String() string {
	return string(p.OS) + "-" + string(p.Arch)
}

// Parse decodes a "<os>-<arch>" string. Any value outside the supported
// matrix is rejected with a clear error.
func Parse(s string) (Platform, error) {
	os_, arch, ok := strings.Cut(s, "-")
	if !ok {
		return Platform{}, fmt.Errorf("invalid platform %q: expected <os>-<arch>", s)
	}
	p := Platform{OS: OS(os_), Arch: Arch(arch)}
	if !p.Supported() {
		return Platform{}, fmt.Errorf("unsupported platform %q: must be one of %s", s, supportedList())
	}
	return p, nil
}

// Supported reports whether p is in the official matrix.
func (p Platform) Supported() bool {
	for _, q := range All() {
		if p == q {
			return true
		}
	}
	return false
}

// Current returns the platform conch is running on, or an error if the
// host's GOOS/GOARCH falls outside the supported matrix.
func Current() (Platform, error) {
	p := Platform{OS: OS(runtime.GOOS), Arch: Arch(runtime.GOARCH)}
	if !p.Supported() {
		return Platform{}, fmt.Errorf(
			"conch does not support %s/%s; supported: %s",
			runtime.GOOS, runtime.GOARCH, supportedList(),
		)
	}
	return p, nil
}

// PowerShellOS returns the segment PowerShell uses for this OS in its
// release artefacts: "win" for windows, "linux" for linux.
func (p Platform) PowerShellOS() string {
	switch p.OS {
	case Windows:
		return "win"
	case Linux:
		return "linux"
	}
	return string(p.OS)
}

// PowerShellArch returns the segment PowerShell uses for this arch in its
// release artefacts: "x64" for amd64, "arm64" for arm64.
func (p Platform) PowerShellArch() string {
	switch p.Arch {
	case AMD64:
		return "x64"
	case ARM64:
		return "arm64"
	}
	return string(p.Arch)
}

// PowerShellExt returns the file extension PowerShell uses for the
// portable archive on this platform: ".zip" on windows, ".tar.gz" on
// linux.
func (p Platform) PowerShellExt() string {
	if p.OS == Windows {
		return ".zip"
	}
	return ".tar.gz"
}

func supportedList() string {
	parts := make([]string, 0, len(All()))
	for _, p := range All() {
		parts = append(parts, p.String())
	}
	return strings.Join(parts, ", ")
}
