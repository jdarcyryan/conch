package pwsh

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/jdarcyryan/conch/internal/version"
)

// Release is a single PowerShell release as far as conch cares: a tag
// (e.g. "v7.5.6") and the parsed version it carries.
type Release struct {
	Tag     string
	Version version.Version
}

// defaultClient returns an HTTP client tuned for talking to the GitHub
// API and download endpoints — modest timeout, no global state. Used
// when a Resolver/Installer is constructed without an explicit HTTP
// override.
func defaultClient() *http.Client {
	return &http.Client{Timeout: 30 * time.Second}
}

// Resolver resolves a version specifier to a concrete PowerShell
// release. It uses the GitHub releases API; an exact specifier short-
// circuits the network call.
type Resolver struct{}

// Resolve picks the newest PowerShell release that satisfies spec.
// "*"/"latest" hits /releases/latest; everything else lists /releases
// and applies the matcher.
func (Resolver) Resolve(spec version.Spec) (Release, error) {
	client := defaultClient()

	if spec.IsAny() {
		return fetchLatest(client)
	}

	releases, err := fetchAllReleases(client)
	if err != nil {
		return Release{}, err
	}
	candidates := make([]version.Version, len(releases))
	for i, rel := range releases {
		candidates[i] = rel.Version
	}
	picked, ok := spec.Pick(candidates)
	if !ok {
		return Release{}, fmt.Errorf("no PowerShell release matches %q", spec.Raw())
	}
	for _, rel := range releases {
		if rel.Version.String() == picked.String() {
			return rel, nil
		}
	}
	return Release{}, fmt.Errorf("internal: matched version %s not found in releases", picked)
}

type ghRelease struct {
	TagName    string `json:"tag_name"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
}

func fetchLatest(client *http.Client) (Release, error) {
	var rel ghRelease
	if err := getJSON(client, releaseLatestURL, &rel); err != nil {
		return Release{}, fmt.Errorf("fetch latest release: %w", err)
	}
	return parseRelease(rel)
}

func fetchAllReleases(client *http.Client) ([]Release, error) {
	var raw []ghRelease
	if err := getJSON(client, releasesListURL, &raw); err != nil {
		return nil, fmt.Errorf("fetch releases: %w", err)
	}
	out := make([]Release, 0, len(raw))
	for _, r := range raw {
		if r.Draft {
			continue
		}
		rel, err := parseRelease(r)
		if err != nil {
			// Skip releases we can't parse — exotic prerelease tags
			// shouldn't break resolution of the rest.
			continue
		}
		out = append(out, rel)
	}
	return out, nil
}

func parseRelease(r ghRelease) (Release, error) {
	tag := r.TagName
	v, err := version.ParseVersion(strings.TrimPrefix(tag, "v"))
	if err != nil {
		return Release{}, fmt.Errorf("parse tag %q: %w", tag, err)
	}
	return Release{Tag: tag, Version: v}, nil
}

func getJSON(client *http.Client, url string, dst any) error {
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GET %s: %s: %s", url, resp.Status, truncate(body, 200))
	}
	return json.NewDecoder(resp.Body).Decode(dst)
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}
