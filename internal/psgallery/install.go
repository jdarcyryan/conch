package psgallery

import (
	"archive/zip"
	"crypto/sha512"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/jdarcyryan/conch/internal/cache"
	"github.com/jdarcyryan/conch/internal/ui"
)

// Installer downloads .nupkg files into the cache (verifying SHA-512
// against PSGallery's published hash on first fetch) and extracts them
// into the conch environment's modules directory.
//
// UI, when non-nil, receives byte-progress updates for each download.
type Installer struct {
	Cache *cache.Cache
	UI    *ui.UI
}

// Install downloads pkg into the cache, verifies its hash, and
// extracts the module files under modulesDir/<Name>/<Version>/. Files
// with .nupkg internal metadata (rels, content types, package metadata)
// are not copied — only the actual module content. Returns the
// SHA-256 of the downloaded .nupkg for the lockfile.
//
// Trust-anchor resolution, in order:
//
//  1. expectedSHA256 (typically from conch.lock) — when non-empty,
//     authoritative: the cache short-circuits on a matching file.
//  2. pkg.SHA512Base64 from the freshly-fetched PSGallery feed — the
//     cached .nupkg is hashed and compared, and on match we skip the
//     download. On mismatch (or no cached file) we fetch fresh,
//     verifying SHA-512 inline.
//  3. No hash at all — fetcher runs unconditionally, no integrity
//     guarantee beyond TLS. Should never happen against a
//     well-behaved PSGallery instance, but kept as a safe last
//     resort.
//
// In every branch the cache is content-addressable storage; it's
// never the source of truth.
func (in *Installer) Install(pkg Package, modulesDir, expectedSHA256 string) (string, error) {
	client := defaultClient()

	cacheName := fmt.Sprintf("%s.%s.nupkg",
		strings.ToLower(pkg.Name), strings.ToLower(pkg.Version.String()))
	fetcher := func(w io.Writer) error { return downloadAndVerify(client, pkg, w, in.UI) }

	archive, err := in.fetchOrCacheHit(cacheName, expectedSHA256, pkg.SHA512Base64, fetcher)
	if err != nil {
		return "", err
	}

	moduleDir := filepath.Join(modulesDir, pkg.Name, pkg.Version.String())
	if err := os.RemoveAll(moduleDir); err != nil {
		return "", fmt.Errorf("clean %s: %w", moduleDir, err)
	}
	if err := os.MkdirAll(moduleDir, 0o755); err != nil {
		return "", fmt.Errorf("create %s: %w", moduleDir, err)
	}
	if err := extractModule(archive, moduleDir); err != nil {
		return "", fmt.Errorf("extract %s: %w", pkg.Name, err)
	}

	hash, err := cache.Hash(archive)
	if err != nil {
		return "", err
	}
	return hash, nil
}

// fetchOrCacheHit resolves a cached .nupkg path by trying, in order:
// the lockfile-supplied SHA-256 (delegated to cache.Get), the
// PSGallery-supplied SHA-512 (verified directly against the cached
// bytes), and finally an unconditional fetch.
//
// Each branch goes through the cache for atomic write semantics; the
// distinction is whether we get to skip the network because we
// already have a hash to compare against.
func (in *Installer) fetchOrCacheHit(cacheName, sha256Hex, sha512B64 string, fetch cache.Fetcher) (string, error) {
	if sha256Hex != "" {
		return in.Cache.Get("modules", cacheName, sha256Hex, fetch)
	}

	cachePath, err := in.Cache.Path("modules", cacheName)
	if err != nil {
		return "", err
	}
	if sha512B64 != "" {
		ok, err := fileMatchesSHA512(cachePath, sha512B64)
		if err != nil {
			return "", err
		}
		if ok {
			return cachePath, nil
		}
	}
	// Cache missed — let the fetcher (which verifies SHA-512 inline)
	// download fresh. cache.Get with empty hash always invokes the
	// fetcher per its updated contract.
	return in.Cache.Get("modules", cacheName, "", fetch)
}

// fileMatchesSHA512 reports whether path exists and hashes (SHA-512,
// Base64) to expected. A missing file is not an error — caller
// proceeds to fetch.
func fileMatchesSHA512(path, expectedBase64 string) (bool, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("stat %s: %w", path, err)
	}
	defer f.Close()
	h := sha512.New()
	if _, err := io.Copy(h, f); err != nil {
		return false, fmt.Errorf("hash %s: %w", path, err)
	}
	return base64.StdEncoding.EncodeToString(h.Sum(nil)) == expectedBase64, nil
}

// downloadAndVerify streams the .nupkg from PSGallery into w while
// computing SHA-512, then compares against pkg.SHA512Base64. If
// SHA512Base64 is empty (e.g. PSGallery returned an unknown algorithm),
// integrity is trusted to TLS — log loudly elsewhere.
//
// When u is non-nil it receives byte-progress updates so the caller
// can render a bar.
func downloadAndVerify(client *http.Client, pkg Package, w io.Writer, u *ui.UI) error {
	if pkg.URL == "" {
		return fmt.Errorf("package %s %s has no download URL", pkg.Name, pkg.Version)
	}
	resp, err := client.Get(pkg.URL)
	if err != nil {
		return fmt.Errorf("GET %s: %w", pkg.URL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("GET %s: %s", pkg.URL, resp.Status)
	}

	hasher := sha512.New()
	writers := []io.Writer{w, hasher}
	var dl *ui.Download
	if u != nil {
		dl = u.BeginDownload(fmt.Sprintf("%s %s", pkg.Name, pkg.Version), resp.ContentLength)
		writers = append(writers, dl)
	}

	if _, err := io.Copy(io.MultiWriter(writers...), resp.Body); err != nil {
		if dl != nil {
			dl.Fail(err)
		}
		return fmt.Errorf("download %s: %w", pkg.URL, err)
	}

	if pkg.SHA512Base64 != "" {
		got := base64.StdEncoding.EncodeToString(hasher.Sum(nil))
		if got != pkg.SHA512Base64 {
			if dl != nil {
				dl.Fail(fmt.Errorf("hash mismatch"))
			}
			return fmt.Errorf(
				"SHA-512 mismatch for %s %s: got %s, expected %s",
				pkg.Name, pkg.Version, got, pkg.SHA512Base64,
			)
		}
	}
	if dl != nil {
		dl.Done()
	}
	return nil
}

// extractModule unzips a .nupkg into dest, dropping the NuGet-internal
// scaffolding (`_rels/`, `package/`, `[Content_Types].xml`). What's
// left is exactly what PSModulePath consumers expect: the module
// manifest, the .psm1, and any supporting files.
func extractModule(archive, dest string) error {
	r, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		if shouldSkipNupkgEntry(f.Name) {
			continue
		}
		// .nupkg paths are URL-escaped (spaces become %20). Undo so
		// disk paths match what users see in PSGallery.
		unescaped, err := url.QueryUnescape(f.Name)
		if err != nil {
			unescaped = f.Name
		}
		path, err := safeJoin(dest, unescaped)
		if err != nil {
			return err
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(path, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			rc.Close()
			return err
		}
		if _, err := io.Copy(out, rc); err != nil {
			rc.Close()
			out.Close()
			return err
		}
		rc.Close()
		out.Close()
	}
	return nil
}

func shouldSkipNupkgEntry(name string) bool {
	switch {
	case strings.HasPrefix(name, "_rels/"),
		strings.HasPrefix(name, "package/"),
		name == "[Content_Types].xml":
		return true
	}
	// Skip the embedded .nuspec — module discovery uses the .psd1
	// manifest, not the NuGet metadata.
	if strings.HasSuffix(strings.ToLower(name), ".nuspec") &&
		!strings.Contains(name, "/") {
		return true
	}
	return false
}

func safeJoin(dest, name string) (string, error) {
	clean := filepath.Clean(name)
	if strings.HasPrefix(clean, "..") || strings.Contains(clean, ".."+string(filepath.Separator)) || filepath.IsAbs(clean) {
		return "", fmt.Errorf("unsafe archive entry %q", name)
	}
	return filepath.Join(dest, clean), nil
}
