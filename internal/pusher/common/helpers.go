package common

import (
	"fmt"
	"strings"
)

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
	case "Mastodon":
		splits := strings.Split(author, "@")
		return fmt.Sprintf("https://%v/@%v/%v", splits[1], splits[0], networkId), nil
	default:
		return "", fmt.Errorf("network %v not recognized", network)
	}
}
