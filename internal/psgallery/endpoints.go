package psgallery

// PSGallery v2 (NuGet OData) endpoints conch reaches when resolving
// and downloading PowerShell modules.
//
// As with internal/pwsh: CLAUDE.md asks for these to live in a
// dedicated configuration layer rather than embedded throughout the
// codebase. Until that layer exists, every URL conch needs lives here
// so it can be lifted out in one move.
const (
	// findPackagesByIDFmt lists every available version of a module.
	// Substitution: %s = url-encoded module id.
	findPackagesByIDFmt = "https://www.powershellgallery.com/api/v2/FindPackagesById()?id='%s'"
)
