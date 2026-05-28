// SPDX-License-Identifier: AGPL-3.0-only
package helpers

import (
	"fmt"
	"strings"
)

type SourceNetwork struct {
	Name                string
	Color               string
	EngagementSupported bool
	ViewsSupported      bool
	FollowersTracked    bool
}

type TargetNetwork struct {
	Name  string
	Color string
}

var AvailableSources = []SourceNetwork{
	{Name: "Instagram", Color: "#ff0076", EngagementSupported: true, ViewsSupported: true, FollowersTracked: true},
	{Name: "Twitter", Color: "#1d9bf0", EngagementSupported: true, ViewsSupported: true, FollowersTracked: true},
	{Name: "Threads", Color: "#000000", EngagementSupported: true, ViewsSupported: true, FollowersTracked: true},
	{Name: "Bluesky", Color: "#1185fe", EngagementSupported: true, ViewsSupported: false, FollowersTracked: true},
	{Name: "YouTube", Color: "#ff0033", EngagementSupported: true, ViewsSupported: true, FollowersTracked: true},
	{Name: "TikTok", Color: "#fe2c55", EngagementSupported: true, ViewsSupported: true, FollowersTracked: true},
	{Name: "Twitch", Color: "#9146ff", EngagementSupported: false, ViewsSupported: true, FollowersTracked: true},
	{Name: "Reddit", Color: "#ff4500", EngagementSupported: true, ViewsSupported: false, FollowersTracked: true},
	{Name: "Mastodon", Color: "#563acc", EngagementSupported: true, ViewsSupported: false, FollowersTracked: true},
	{Name: "Discord", Color: "#5662f6", EngagementSupported: true, ViewsSupported: false, FollowersTracked: true},
	{Name: "Telegram", Color: "#26a4e3", EngagementSupported: true, ViewsSupported: true, FollowersTracked: true},
	{Name: "Google Analytics", Color: "#e37400", EngagementSupported: false, ViewsSupported: false, FollowersTracked: false},
	{Name: "Google Search Console", Color: "#4285F4", EngagementSupported: false, ViewsSupported: false, FollowersTracked: false},
	{Name: "BadPups", Color: "#c1272d", EngagementSupported: true, ViewsSupported: true, FollowersTracked: true},
	{Name: "Murrtube", Color: "#344aa8", EngagementSupported: true, ViewsSupported: true, FollowersTracked: true},
	{Name: "DeviantArt", Color: "#24e39d", EngagementSupported: true, ViewsSupported: true, FollowersTracked: true},
	{Name: "e621", Color: "#01549b", EngagementSupported: true, ViewsSupported: false, FollowersTracked: false},
	{Name: "Weasyl", Color: "#990000", EngagementSupported: true, ViewsSupported: true, FollowersTracked: true},
	{Name: "FurTrack", Color: "#2d0e4c", EngagementSupported: true, ViewsSupported: false, FollowersTracked: false},
	{Name: "FurAffinity", Color: "#f9af3B", EngagementSupported: true, ViewsSupported: true, FollowersTracked: true},
}

func GetSourceByName(name string) *SourceNetwork {
	for i := range AvailableSources {
		if AvailableSources[i].Name == name {
			return &AvailableSources[i]
		}
	}
	return nil
}

var AvailableTargets = []TargetNetwork{
	{Name: "NocoDB", Color: "#4351e8"},
	{Name: "CSV", Color: "#45b058"},
}

func ConvNetworkToURL(network, username string) (string, error) {
	switch network {
	case "Instagram":
		return "https://instagram.com/" + username, nil
	case "Bluesky":
		return "https://bsky.app/profile/" + username, nil
	case "TikTok":
		return "https://tiktok.com/@" + username, nil
	case "BadPups":
		return "https://badpups.com/lite/profile/" + username, nil
	case "Murrtube":
		return "https://murrtube.net/" + username, nil
	case "FurTrack":
		return "https://www.furtrack.com/user/" + username + "/photography", nil
	case "Telegram":
		return "https://t.me/" + username, nil
	case "YouTube":
		return "https://youtube.com/" + username, nil
	case "Discord":
		return "https://discord.com/channels/" + username, nil
	case "Mastodon":
		splits := strings.Split(username, "@")
		return fmt.Sprintf("https://%v/@%v", splits[1], splits[0]), nil
	case "Google Analytics":
		return "analytics.google.com/analytics/web/", nil
	case "FurAffinity":
		return "https://www.furaffinity.net/user/" + username + "/", nil
	case "e621":
		return "https://e621.net/posts?tags=user:" + username, nil
	case "Reddit":
		return "https://reddit.com/user/" + username, nil
	case "Twitch":
		return "https://twitch.tv/" + username, nil
	case "Twitter":
		return "https://twitter.com/" + username, nil
	case "Threads":
		return "https://www.threads.net/@" + username, nil
	case "DeviantArt":
		return "https://www.deviantart.com/" + username, nil
	case "Weasyl":
		return "https://www.weasyl.com/~" + username, nil
	case "Google Search Console":
		return "https://search.google.com/search-console/", nil
	default:
		return "", fmt.Errorf("network %v not recognized", network)
	}
}

func ConvPostToURL(network, author, networkId string) (string, error) {
	switch network {
	case "Instagram":
		return "https://instagram.com/p/" + networkId, nil
	case "Bluesky":
		return "https://bsky.app/profile/" + author + "/post/" + networkId, nil
	case "TikTok":
		return "https://www.tiktok.com/@" + author + "/video/" + networkId, nil
	case "BadPups":
		return "https://badpups.com/lite/video/" + networkId, nil
	case "Murrtube":
		return "https://murrtube.net/v/" + networkId, nil
	case "Telegram":
		return "https://t.me/" + author + "/" + networkId, nil
	case "YouTube":
		return "https://youtube.com/watch?v=" + networkId, nil
	case "FurTrack":
		return "https://www.furtrack.com/user/" + author + "/album-" + networkId, nil
	case "Discord":
		parts := strings.Split(networkId, "/")
		if len(parts) == 3 {
			return "https://discord.com/channels/" + parts[0] + "/" + parts[1] + "/" + parts[2], nil
		}
		return "", fmt.Errorf("invalid Discord message ID format")
	case "Mastodon":
		splits := strings.Split(author, "@")
		return fmt.Sprintf("https://%v/@%v/%v", splits[1], splits[0], networkId), nil
	case "FurAffinity":
		return "https://www.furaffinity.net/view/" + networkId + "/", nil
	case "e621":
		return "https://e621.net/posts/" + networkId, nil
	case "Reddit":
		return "https://reddit.com/comments/" + networkId, nil
	case "Twitch":
		isNumeric := len(networkId) > 0
		for _, ch := range networkId {
			if ch < '0' || ch > '9' {
				isNumeric = false
				break
			}
		}
		if isNumeric {
			return "https://www.twitch.tv/videos/" + networkId, nil
		}
		return "https://www.twitch.tv/" + author + "/clip/" + networkId, nil
	case "Twitter":
		return "https://twitter.com/" + author + "/status/" + networkId, nil
	case "Threads":
		return "https://www.threads.net/@" + author + "/post/" + networkId, nil
	case "DeviantArt":
		return "https://www.deviantart.com/" + author + "/art/" + networkId, nil
	case "Weasyl":
		return "https://www.weasyl.com/~" + author + "/submissions/" + networkId, nil
	default:
		return "", fmt.Errorf("network %v not recognized", network)
	}
}
