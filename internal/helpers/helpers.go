package helpers

import (
	"fmt"
	"strings"
)

type SourceNetwork struct {
	Name  string
	Color string
}

type TargetNetwork struct {
	Name  string
	Color string
}

var AvailableSources = []SourceNetwork{
	{Name: "Instagram", Color: "#ff0076"},
	{Name: "Bluesky", Color: "#1185fe"},
	{Name: "YouTube", Color: "#ff0033"},
	{Name: "TikTok", Color: "#fe2c55"},
	{Name: "Mastodon", Color: "#563acc"},
	{Name: "Telegram", Color: "#26a4e3"},
	{Name: "Google Analytics", Color: "#e37400"},
	{Name: "BadPups", Color: "#c1272d"},
	{Name: "Murrtube", Color: "#344aa8"},
	{Name: "Discord", Color: "#5662f6"},
	{Name: "FurTrack", Color: "#2d0e4c"},
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
	default:
		return "", fmt.Errorf("network %v not recognized", network)
	}
}
