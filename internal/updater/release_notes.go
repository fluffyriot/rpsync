// SPDX-License-Identifier: AGPL-3.0-only
package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type GitHubRelease struct {
	Body    string `json:"body"`
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	HtmlUrl string `json:"html_url"`
}

func (u *Updater) GetReleaseNotes(fromVersion, toVersion string, limit int) (*GitHubRelease, error) {
	url := "https://api.github.com/repos/fluffyriot/rpsync/releases"

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch releases: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(body))
	}

	var releases []GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		return nil, fmt.Errorf("failed to decode releases: %w", err)
	}

	if len(releases) == 0 {
		return &GitHubRelease{Body: "No release notes available.", TagName: toVersion, Name: "v." + toVersion}, nil
	}

	var relevantReleases []GitHubRelease

	if limit > 0 {
		count := 0
		for _, rel := range releases {
			v := strings.TrimPrefix(rel.TagName, "v.")
			if !u.isNewer(v, toVersion) {
				relevantReleases = append(relevantReleases, rel)
				count++
				if count >= limit {
					break
				}
			}
		}
	} else {
		if toVersion == "unknown" || toVersion == "" {
			if len(releases) > 0 {
				toVersion = strings.TrimPrefix(releases[0].TagName, "v.")
			}
		}

		for _, rel := range releases {
			v := strings.TrimPrefix(rel.TagName, "v.")

			isNewerThanFrom := u.isNewer(v, fromVersion)
			isOlderOrEqualToTo := !u.isNewer(v, toVersion)

			if isNewerThanFrom && isOlderOrEqualToTo {
				relevantReleases = append(relevantReleases, rel)
			}
		}
	}

	if len(relevantReleases) == 0 {
		return &GitHubRelease{Body: "No new release notes found for this range.", TagName: toVersion, Name: "v." + toVersion}, nil
	}
	var combinedBody strings.Builder
	var combinedName string
	var combinedHtmlUrl string

	for i, rel := range relevantReleases {
		if i > 0 {
			combinedBody.WriteString("\n\n---\n\n")
		}
		combinedBody.WriteString(fmt.Sprintf("# %s\n\n%s", rel.Name, rel.Body))

		if i == 0 {
			combinedName = rel.Name
			combinedHtmlUrl = rel.HtmlUrl
		}
	}

	if len(relevantReleases) > 1 {
		combinedName = fmt.Sprintf("Updates up to %s", relevantReleases[0].TagName)
	}

	return &GitHubRelease{
		Body:    combinedBody.String(),
		TagName: relevantReleases[0].TagName,
		Name:    combinedName,
		HtmlUrl: combinedHtmlUrl,
	}, nil
}
