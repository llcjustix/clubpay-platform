// Package release discovers immutable ClubPay build artifacts. It deliberately
// consumes the GitHub Releases API rather than guessing a "latest" redirect,
// so an updater always knows the exact tag and checksum it is about to run.
package release

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type Artifact struct {
	Version     string
	DownloadURL string
	ChecksumURL string
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

type githubRelease struct {
	TagName    string        `json:"tag_name"`
	Draft      bool          `json:"draft"`
	Prerelease bool          `json:"prerelease"`
	Assets     []githubAsset `json:"assets"`
}

// Latest returns the newest stable release whose tag has tagPrefix and which
// includes both the requested archive and its SHA-256 sidecar.
func Latest(ctx context.Context, client *http.Client, apiURL, tagPrefix, archiveName string) (Artifact, error) {
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return Artifact{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	res, err := client.Do(req)
	if err != nil {
		return Artifact{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return Artifact{}, fmt.Errorf("release lookup returned HTTP %d", res.StatusCode)
	}
	var releases []githubRelease
	if err := json.NewDecoder(res.Body).Decode(&releases); err != nil {
		return Artifact{}, err
	}
	var best Artifact
	for _, item := range releases {
		if item.Draft || item.Prerelease || !strings.HasPrefix(item.TagName, tagPrefix) {
			continue
		}
		candidate := Artifact{Version: item.TagName}
		for _, asset := range item.Assets {
			switch asset.Name {
			case archiveName:
				candidate.DownloadURL = asset.BrowserDownloadURL
			case archiveName + ".sha256":
				candidate.ChecksumURL = asset.BrowserDownloadURL
			}
		}
		if candidate.DownloadURL == "" || candidate.ChecksumURL == "" {
			continue
		}
		if best.Version == "" || CompareVersions(candidate.Version, best.Version) > 0 {
			best = candidate
		}
	}
	if best.Version == "" {
		return Artifact{}, fmt.Errorf("no complete release found for %s", archiveName)
	}
	return best, nil
}

// CompareVersions compares controller-v0.2.26, v0.4.25 and similar semantic
// release tags. Non-numeric suffixes are intentionally rejected as older than
// the corresponding stable release.
func CompareVersions(left, right string) int {
	leftParts, leftOK := versionParts(left)
	rightParts, rightOK := versionParts(right)
	if !leftOK || !rightOK {
		return strings.Compare(left, right)
	}
	max := len(leftParts)
	if len(rightParts) > max {
		max = len(rightParts)
	}
	for i := 0; i < max; i++ {
		l, r := 0, 0
		if i < len(leftParts) {
			l = leftParts[i]
		}
		if i < len(rightParts) {
			r = rightParts[i]
		}
		if l < r {
			return -1
		}
		if l > r {
			return 1
		}
	}
	return 0
}

func versionParts(value string) ([]int, bool) {
	value = strings.TrimPrefix(strings.TrimSpace(value), "controller-")
	value = strings.TrimPrefix(value, "v")
	if value == "" {
		return nil, false
	}
	parts := strings.Split(value, ".")
	parsed := make([]int, len(parts))
	for i, part := range parts {
		if part == "" {
			return nil, false
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 0 {
			return nil, false
		}
		parsed[i] = n
	}
	return parsed, true
}
