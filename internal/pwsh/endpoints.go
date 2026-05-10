package pwsh

// Remote endpoints conch reaches when resolving and downloading
// PowerShell distributions.
//
// CLAUDE.md calls for these to live in a dedicated configuration layer
// rather than embedded throughout the codebase. Until that layer
// exists, every URL conch needs lives here so it can be lifted out in
// one move.
const (
	// releaseDownloadFmt is the URL template for an artefact published
	// against a given PowerShell release tag. Substitutions: %s = tag
	// (e.g. "v7.5.6"), %s = filename.
	releaseDownloadFmt = "https://github.com/PowerShell/PowerShell/releases/download/%s/%s"

	// releasesListURL lists every PowerShell release. Used when
	// resolving wildcard or range specifiers.
	releasesListURL = "https://api.github.com/repos/PowerShell/PowerShell/releases?per_page=100"

	// releaseLatestURL returns the latest stable release. Used when the
	// specifier is "*" or "latest".
	releaseLatestURL = "https://api.github.com/repos/PowerShell/PowerShell/releases/latest"

	// hashesFilename is the SHA-256 manifest published alongside each
	// release. The file is encoded as UTF-16 LE with BOM, which is why
	// hashes.go has its own decoder rather than using string(body).
	hashesFilename = "hashes.sha256"
)
