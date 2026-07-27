package psgallery

import (
	"os"
	"strings"
)

// PSGallery v2 (NuGet OData) endpoints conch reaches when resolving
// and downloading PowerShell modules.
//
// As with internal/pwsh: CLAUDE.md asks for these to live in a
// dedicated configuration layer rather than embedded throughout the
// codebase. Until that layer exists, every URL conch needs lives here
// so it can be lifted out in one move.
const (
	// psGalleryBaseURL is the default NuGet v2 feed used when no
	// override is configured.
	psGalleryBaseURL = "https://www.powershellgallery.com/api/v2"

	// findPackagesByIDFmt lists every available version of a module.
	// Substitutions: %s = feed base URL, %s = url-encoded module id.
	findPackagesByIDFmt = "%s/FindPackagesById()?id='%s'"
)

// Environment variables that override the module feed.
const (
	// envFeedURL points conch at an alternative NuGet v2 feed instead
	// of the public PSGallery.
	envFeedURL = "CONCH_NUGET_URL"

	// envFeedAPIKey is the API key sent with requests to the feed
	// named by envFeedURL. It is never sent to the PSGallery fallback.
	envFeedAPIKey = "CONCH_NUGET_API_KEY"
)

// feedBaseURL returns the NuGet v2 feed base URL: CONCH_NUGET_URL when
// set, otherwise the public PSGallery feed.
func feedBaseURL() string {
	if v := os.Getenv(envFeedURL); v != "" {
		return strings.TrimRight(v, "/")
	}
	return psGalleryBaseURL
}

// feedAPIKey returns the API key to attach to feed requests. The key
// is only honoured alongside CONCH_NUGET_URL — when conch falls back
// to the public PSGallery no key is sent, whatever the environment
// says.
func feedAPIKey() string {
	if os.Getenv(envFeedURL) == "" {
		return ""
	}
	return os.Getenv(envFeedAPIKey)
}
