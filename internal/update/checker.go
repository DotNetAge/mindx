package update

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	githubRepo = "DotNetAge/mindx"
	githubAPI  = "https://api.github.com/repos/" + githubRepo + "/releases/latest"
)

// ReleaseInfo represents a GitHub release.
type ReleaseInfo struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// LatestRelease fetches the latest release info from GitHub.
func LatestRelease(httpClient *http.Client) (*ReleaseInfo, error) {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}

	req, err := http.NewRequest("GET", githubAPI, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch latest release: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		// Read body for rate limit info
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API returned %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var rel ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, fmt.Errorf("decode release: %w", err)
	}

	return &rel, nil
}

// CompareVersions compares two semver strings (e.g. "2.1.0").
// Returns:
//
//	-1 if v1 < v2
//	 0 if v1 == v2
//	 1 if v1 > v2
func CompareVersions(v1, v2 string) int {
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	var maj1, min1, pat1 int
	var maj2, min2, pat2 int

	n1, _ := fmt.Sscanf(v1, "%d.%d.%d", &maj1, &min1, &pat1)
	n2, _ := fmt.Sscanf(v2, "%d.%d.%d", &maj2, &min2, &pat2)

	if n1 < 3 || n2 < 3 {
		// Fallback: compare as strings if we can't parse
		return strings.Compare(v1, v2)
	}

	if maj1 != maj2 {
		return cmpInt(maj1, maj2)
	}
	if min1 != min2 {
		return cmpInt(min1, min2)
	}
	return cmpInt(pat1, pat2)
}

func cmpInt(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

// IsNewer returns true if the latest version is newer than the current version.
func IsNewer(latestTag, currentVersion string) bool {
	latest := strings.TrimPrefix(latestTag, "v")
	current := strings.TrimPrefix(currentVersion, "v")
	return CompareVersions(latest, current) > 0
}

// FindAssetForPlatform finds a release asset matching the given OS and arch.
func (r *ReleaseInfo) FindAssetForPlatform(os, arch string) (string, string, error) {
	var suffix string
	switch os {
	case "darwin", "linux":
		suffix = fmt.Sprintf("%s-%s.tar.gz", os, arch)
	case "windows":
		suffix = fmt.Sprintf("windows-%s.zip", arch)
	default:
		return "", "", fmt.Errorf("unsupported OS: %s", os)
	}

	for _, asset := range r.Assets {
		if strings.HasSuffix(asset.Name, suffix) && strings.Contains(asset.Name, "-"+strings.TrimPrefix(r.TagName, "v")+"-") {
			return asset.BrowserDownloadURL, asset.Name, nil
		}
	}
	return "", "", fmt.Errorf("no release asset found for %s/%s", os, arch)
}
