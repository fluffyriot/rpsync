package updater

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	VersionJsonUrl = "https://raw.githubusercontent.com/fluffyriot/rpsync/main/version.json"
	CheckInterval  = 6 * time.Hour
)

type RemoteVersion struct {
	Latest string `json:"latest"`
}

type Updater struct {
	mu              sync.RWMutex
	updateAvailable bool
	remoteVersion   RemoteVersion
	currentVersion  string
	checkInterval   time.Duration
}

func NewUpdater(currentVersion string) *Updater {
	return &Updater{
		currentVersion: currentVersion,
		checkInterval:  CheckInterval,
	}
}

func (u *Updater) Start() {
	go u.Check()

	go func() {
		ticker := time.NewTicker(u.checkInterval)
		defer ticker.Stop()
		for range ticker.C {
			u.Check()
		}
	}()
}

func (u *Updater) Check() {
	log.Println("Checking for updates...")
	resp, err := http.Get(VersionJsonUrl)
	if err != nil {
		log.Printf("Failed to check for updates: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Failed to check for updates: status code %d", resp.StatusCode)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read update response: %v", err)
		return
	}

	var rv RemoteVersion
	if err := json.Unmarshal(body, &rv); err != nil {
		log.Printf("Failed to unmarshal update response: %v", err)
		return
	}

	u.mu.Lock()
	defer u.mu.Unlock()

	u.remoteVersion = rv

	if u.isNewer(rv.Latest, u.currentVersion) {
		u.updateAvailable = true
		log.Printf("New version available: %s (current: %s)", rv.Latest, u.currentVersion)
	} else {
		u.updateAvailable = false
		log.Printf("App is up to date (current: %s, latest: %s)", u.currentVersion, rv.Latest)
	}
}

func (u *Updater) isNewer(remote, current string) bool {

	if remote == current || current == "unknown" {
		return false
	}

	rParts := strings.Split(remote, ".")
	cParts := strings.Split(current, ".")

	maxLen := len(rParts)
	if len(cParts) < maxLen {
		maxLen = len(cParts)
	}

	for i := 0; i < maxLen; i++ {
		var rVal, cVal int
		if i < len(rParts) {
			fmt.Sscanf(rParts[i], "%d", &rVal)
		}
		if i < len(cParts) {
			fmt.Sscanf(cParts[i], "%d", &cVal)
		}

		if rVal > cVal {
			return true
		}
		if rVal < cVal {
			return false
		}
	}

	return len(rParts) > len(cParts)
}

func (u *Updater) IsUpdateAvailable() bool {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.updateAvailable
}

func (u *Updater) GetUpdateInfo() RemoteVersion {
	u.mu.RLock()
	defer u.mu.RUnlock()
	return u.remoteVersion
}
