// Package cache implements conch's shared download cache.
//
// Every artefact conch fetches — PowerShell archives, module .nupkg
// files — passes through this cache, which lives at a well-known
// per-user location and is shared across projects. The flow is the
// same in every case:
//
//  1. If a file with the expected name is already present, hash it.
//  2. If the hash matches the expected SHA-256, the cached file wins.
//  3. Otherwise, fetch it, hash the fresh download, and write it
//     atomically to the cache. A corrupted or tampered cache file is
//     replaced rather than trusted.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Cache is a directory-rooted artefact cache. It is safe to use the
// zero-value via Default().
type Cache struct {
	dir string
}

// Default returns the conch user cache rooted at os.UserCacheDir/conch.
// The directory is created if missing.
func Default() (*Cache, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("locate user cache directory: %w", err)
	}
	return New(filepath.Join(base, "conch"))
}

// New returns a cache rooted at dir. The directory is created if
// missing.
func New(dir string) (*Cache, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create cache dir %s: %w", dir, err)
	}
	return &Cache{dir: dir}, nil
}

// Dir returns the directory backing the cache.
func (c *Cache) Dir() string { return c.dir }

// Path returns the on-disk path a given (subdir, name) pair would
// resolve to. subdir is something like "pwsh" or "modules"; name is
// the file's basename (e.g. "PowerShell-7.5.6-win-x64.zip"). subdir
// is created on demand.
func (c *Cache) Path(subdir, name string) (string, error) {
	full := filepath.Join(c.dir, subdir)
	if err := os.MkdirAll(full, 0o755); err != nil {
		return "", fmt.Errorf("create cache subdir %s: %w", full, err)
	}
	return filepath.Join(full, name), nil
}

// Fetcher fetches an artefact's bytes when the cache misses. It writes
// to w (which is a temp file managed by the cache) and returns a hint
// about the SHA-256 it expected; an empty string means "trust the
// cache's expectedSHA256 only".
type Fetcher func(w io.Writer) error

// Get returns a cached file path for (subdir, name), fetching it via
// fetch when the cached copy is missing or its SHA-256 doesn't match
// expectedSHA256.
//
// expectedSHA256 is the trust anchor for this lookup: a hex-encoded
// digest, compared case-insensitively. It must come from outside the
// cache (the project lockfile, a freshly-fetched upstream manifest,
// etc.) — the cache itself is content-addressable storage, not a
// source of truth.
//
// When expectedSHA256 is empty, the cached copy (if any) is ignored
// and fetch is run unconditionally. Fetchers in that mode are
// responsible for verifying the bytes they write against an external
// authority before declaring success — otherwise a tampered cache
// file would be silently trusted, which is exactly what conch must
// not do.
func (c *Cache) Get(subdir, name, expectedSHA256 string, fetch Fetcher) (string, error) {
	dest, err := c.Path(subdir, name)
	if err != nil {
		return "", err
	}

	if ok, err := matchesHash(dest, expectedSHA256); err != nil {
		return "", err
	} else if ok {
		return dest, nil
	}

	tmp := dest + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return "", fmt.Errorf("create %s: %w", tmp, err)
	}

	hasher := sha256.New()
	mw := io.MultiWriter(f, hasher)

	if err := fetch(mw); err != nil {
		f.Close()
		os.Remove(tmp)
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return "", fmt.Errorf("close %s: %w", tmp, err)
	}

	if expectedSHA256 != "" {
		gotHex := hex.EncodeToString(hasher.Sum(nil))
		if !strings.EqualFold(gotHex, expectedSHA256) {
			os.Remove(tmp)
			return "", fmt.Errorf("hash mismatch for %s: got %s, expected %s", name, gotHex, expectedSHA256)
		}
	}

	if err := os.Rename(tmp, dest); err != nil {
		os.Remove(tmp)
		return "", fmt.Errorf("rename %s → %s: %w", tmp, dest, err)
	}
	return dest, nil
}

// Hash returns the SHA-256 of path as a lowercase hex string.
func Hash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// matchesHash reports whether path exists and hashes to expected. A
// missing file is *not* an error — it just reports false so the
// caller proceeds to download.
//
// An empty expected hash deliberately reports a miss: without a
// trust anchor the cached file cannot be vouched for, so the fetcher
// must run and verify the bytes against an external authority.
func matchesHash(path, expected string) (bool, error) {
	if expected == "" {
		return false, nil
	}
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("stat %s: %w", path, err)
	}
	if info.IsDir() {
		return false, fmt.Errorf("%s is a directory, expected file", path)
	}
	got, err := Hash(path)
	if err != nil {
		return false, fmt.Errorf("hash %s: %w", path, err)
	}
	return strings.EqualFold(got, expected), nil
}
