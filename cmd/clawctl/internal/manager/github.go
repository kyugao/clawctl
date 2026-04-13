package manager

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Release represents a GitHub release.
type Release struct {
	TagName    string    `json:"tag_name"`
	PublishedAt time.Time `json:"published_at"`
	Prerelease bool      `json:"prerelease"`
}

// FetchReleases fetches the list of releases from GitHub API.
// Returns up to limit releases sorted by published_at descending.
// Silently returns nil on error (network, rate limit, etc.).
func FetchReleases(repo string, limit int) ([]Release, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases", repo)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "clawctl")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	// Decode into []Release directly.
	var releases []Release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, err
	}

	if len(releases) > limit {
		releases = releases[:limit]
	}
	return releases, nil
}

// FetchLatestTag fetches the tag name of the latest release.
// Returns empty string on error.
func FetchLatestTag(repo string) (string, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "clawctl")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}

	var rel struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", err
	}
	return rel.TagName, nil
}
