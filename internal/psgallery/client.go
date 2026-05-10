// Package psgallery is a small client for PowerShell Gallery's v2 API
// (the NuGet OData feed used by PowerShellGet v2). It is the minimum
// surface conch needs: list versions, resolve a specifier to a single
// package, and download .nupkg files via the shared cache.
package psgallery

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jdarcyryan/conch/internal/version"
)

// Package is one PSGallery package version, projected to just the
// fields conch cares about.
type Package struct {
	Name         string
	Version      version.Version
	URL          string // download URL for the .nupkg
	SHA512Base64 string // PackageHash from PSGallery (raw bytes, base64-encoded)
}

// defaultClient returns an HTTP client tuned for talking to PSGallery —
// modest timeout, no global state. Used when a Client/Installer is
// constructed without an explicit HTTP override.
func defaultClient() *http.Client {
	return &http.Client{Timeout: 60 * time.Second}
}

// Client is a thin wrapper around an HTTP client that knows the
// OData feed.
type Client struct {
	HTTP *http.Client
}

// Resolve returns the newest package version for name that satisfies
// spec.
func (c *Client) Resolve(name string, spec version.Spec) (Package, error) {
	all, err := c.ListVersions(name)
	if err != nil {
		return Package{}, err
	}
	candidates := make([]version.Version, len(all))
	for i, p := range all {
		candidates[i] = p.Version
	}
	picked, ok := spec.Pick(candidates)
	if !ok {
		return Package{}, fmt.Errorf("no version of %s satisfies %q", name, spec.Raw())
	}
	for _, p := range all {
		if p.Version.String() == picked.String() {
			return p, nil
		}
	}
	return Package{}, fmt.Errorf("internal: matched version %s not found in feed", picked)
}

// ListVersions returns every published version of the given module.
// Pagination is handled transparently — each Atom response carries a
// `<link rel="next" href="...">` if there are more pages.
func (c *Client) ListVersions(name string) ([]Package, error) {
	client := c.HTTP
	if client == nil {
		client = defaultClient()
	}
	out := []Package{}
	next := fmt.Sprintf(findPackagesByIDFmt, url.QueryEscape(name))
	for next != "" {
		feed, err := getFeed(client, next)
		if err != nil {
			return nil, fmt.Errorf("list versions of %s: %w", name, err)
		}
		for _, e := range feed.Entries {
			pkg, err := entryToPackage(name, e)
			if err != nil {
				continue
			}
			out = append(out, pkg)
		}
		next = ""
		for _, l := range feed.Links {
			if l.Rel == "next" {
				next = l.Href
				break
			}
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no versions of %s found on PSGallery", name)
	}
	return out, nil
}

// --- Atom/OData wire types --------------------------------------------------

type atomFeed struct {
	XMLName xml.Name    `xml:"feed"`
	Links   []atomLink  `xml:"link"`
	Entries []atomEntry `xml:"entry"`
}

type atomLink struct {
	Rel  string `xml:"rel,attr"`
	Href string `xml:"href,attr"`
}

type atomEntry struct {
	Content    atomContent     `xml:"content"`
	Properties odataProperties `xml:"properties"`
}

type atomContent struct {
	Src string `xml:"src,attr"`
}

// odataProperties uses local XML names (no namespace) because Go's
// encoding/xml treats element matching as namespace-agnostic when no
// XMLName is set.
type odataProperties struct {
	Version              string `xml:"Version"`
	NormalizedVersion    string `xml:"NormalizedVersion"`
	PackageHash          string `xml:"PackageHash"`
	PackageHashAlgorithm string `xml:"PackageHashAlgorithm"`
}

func getFeed(client *http.Client, url string) (*atomFeed, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GET %s: %s: %s", url, resp.Status, truncate(body, 200))
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	var feed atomFeed
	if err := xml.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("decode atom feed: %w", err)
	}
	return &feed, nil
}

func entryToPackage(name string, e atomEntry) (Package, error) {
	verStr := e.Properties.NormalizedVersion
	if verStr == "" {
		verStr = e.Properties.Version
	}
	v, err := version.ParseVersion(verStr)
	if err != nil {
		return Package{}, err
	}
	pkg := Package{
		Name:         name,
		Version:      v,
		URL:          e.Content.Src,
		SHA512Base64: e.Properties.PackageHash,
	}
	if !strings.EqualFold(e.Properties.PackageHashAlgorithm, "SHA512") {
		// PSGallery has used SHA512 for years; if a future entry
		// changes algorithm, fall through with empty hash so the
		// caller can decide whether to fail or trust.
		pkg.SHA512Base64 = ""
	}
	return pkg, nil
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}
