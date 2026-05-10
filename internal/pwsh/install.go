package pwsh

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/jdarcyryan/conch/internal/cache"
	"github.com/jdarcyryan/conch/internal/platform"
	"github.com/jdarcyryan/conch/internal/ui"
)

// ArtefactName returns the filename of the PowerShell archive for
// (rel, p) — e.g. "PowerShell-7.5.6-win-x64.zip" on Windows or
// "powershell-7.5.6-linux-x64.tar.gz" on Linux. PowerShell's release
// naming is case-sensitive: Windows artefacts use Pascal-case, Linux
// artefacts use lowercase. The hashes manifest contains both forms.
func ArtefactName(rel Release, p platform.Platform) string {
	prefix := "PowerShell"
	if p.OS == platform.Linux {
		prefix = "powershell"
	}
	return fmt.Sprintf("%s-%s-%s-%s%s",
		prefix, rel.Version.String(), p.PowerShellOS(), p.PowerShellArch(), p.PowerShellExt())
}

// DownloadURL returns the GitHub release download URL for an artefact.
func DownloadURL(rel Release, name string) string {
	return fmt.Sprintf(releaseDownloadFmt, rel.Tag, name)
}

// Installer downloads and extracts a PowerShell release into a target
// directory. It uses the shared cache for both the archive and the
// hashes file, and verifies SHA-256 on every download.
//
// UI, when non-nil, receives progress updates for the archive
// download. The (small) hashes file is fetched silently — its size
// is below noise level.
type Installer struct {
	Cache *cache.Cache
	UI    *ui.UI
}

// Install downloads PowerShell rel for platform p and extracts it into
// targetDir. If targetDir already exists it is replaced. Returns the
// SHA-256 of the archive that was used (handy for the lockfile).
//
// expectedSHA256 is the trust anchor for this install:
//
//   - When non-empty, it is treated as authoritative — typically
//     coming from the project's conch.lock for a previously-resolved
//     install. The cache is searched for a file matching the digest;
//     if found, no network round-trip is needed.
//   - When empty, the GitHub-published hashes.sha256 manifest for
//     this release is fetched fresh (never cached) and used to
//     verify the archive. The cache is *not* trusted as a hash
//     source — it only ever holds bytes.
func (in *Installer) Install(rel Release, p platform.Platform, targetDir, expectedSHA256 string) (string, error) {
	client := defaultClient()

	name := ArtefactName(rel, p)
	expectedHash := expectedSHA256
	if expectedHash == "" {
		hashes, err := fetchHashesLive(client, rel)
		if err != nil {
			return "", err
		}
		got, ok := hashes[name]
		if !ok {
			return "", fmt.Errorf("hashes file does not list %s", name)
		}
		expectedHash = got
	}

	archive, err := in.Cache.Get("pwsh", name, expectedHash, func(w io.Writer) error {
		return downloadWithProgress(client, DownloadURL(rel, name), w, in.UI, name)
	})
	if err != nil {
		return "", fmt.Errorf("fetch %s: %w", name, err)
	}

	if err := os.RemoveAll(targetDir); err != nil {
		return "", fmt.Errorf("clean %s: %w", targetDir, err)
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return "", fmt.Errorf("create %s: %w", targetDir, err)
	}

	if strings.HasSuffix(name, ".zip") {
		if err := extractZip(archive, targetDir); err != nil {
			return "", fmt.Errorf("extract %s: %w", name, err)
		}
	} else {
		if err := extractTarGz(archive, targetDir); err != nil {
			return "", fmt.Errorf("extract %s: %w", name, err)
		}
	}
	return expectedHash, nil
}

// fetchHashesLive pulls the hashes.sha256 file for rel directly from
// GitHub on every call — no caching, no on-disk middleman. The hashes
// file is the trust anchor for archives without a lockfile entry; if
// it lived in the cache, an attacker with cache write access could
// simply rewrite both the hashes file and the archive in lockstep.
//
// The file itself is small (a few KB) so re-downloading per install
// is cheap.
func fetchHashesLive(client *http.Client, rel Release) (map[string]string, error) {
	url := fmt.Sprintf(releaseDownloadFmt, rel.Tag, hashesFilename)
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch hashes: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return decodeHashesFile(resp.Body)
}

// downloadWithProgress streams the URL's body into w and reports byte
// progress to u (when non-nil) under the given display name. We pull
// Content-Length off the response so the bar can show a real
// percentage; servers that omit it degrade gracefully to an
// indeterminate spinner inside the UI.
func downloadWithProgress(client *http.Client, url string, w io.Writer, u *ui.UI, name string) error {
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}

	dst := w
	var dl *ui.Download
	if u != nil {
		dl = u.BeginDownload(name, resp.ContentLength)
		dst = io.MultiWriter(w, dl)
	}
	if _, err := io.Copy(dst, resp.Body); err != nil {
		if dl != nil {
			dl.Fail(err)
		}
		return fmt.Errorf("download body %s: %w", url, err)
	}
	if dl != nil {
		dl.Done()
	}
	return nil
}

func extractZip(archive, dest string) error {
	r, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		path, err := safeJoin(dest, f.Name)
		if err != nil {
			return err
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(path, f.Mode().Perm()|0o700); err != nil {
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
		out, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode().Perm()|0o600)
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

func extractTarGz(archive, dest string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		path, err := safeJoin(dest, hdr.Name)
		if err != nil {
			return err
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, os.FileMode(hdr.Mode)|0o700); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode)|0o600)
			if err != nil {
				return err
			}
			if _, err := io.CopyN(out, tr, hdr.Size); err != nil && err != io.EOF {
				out.Close()
				return err
			}
			out.Close()
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			// Best-effort; PowerShell tarballs include some symlinks.
			_ = os.Symlink(hdr.Linkname, path)
		}
	}
}

// safeJoin guards against zip-slip — refuses entries whose path
// escapes dest after cleaning.
func safeJoin(dest, name string) (string, error) {
	clean := filepath.Clean(name)
	if strings.HasPrefix(clean, "..") || strings.Contains(clean, ".."+string(filepath.Separator)) || filepath.IsAbs(clean) {
		return "", fmt.Errorf("unsafe archive entry %q", name)
	}
	return filepath.Join(dest, clean), nil
}
