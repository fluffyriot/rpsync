package puller

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
	case "Mastodon":
		splits := strings.Split(username, "@")
		return fmt.Sprintf("https://%v/@%v", splits[1], splits[0]), nil
	default:
		return "", fmt.Errorf("network %v not recognized", network)
	}
}
