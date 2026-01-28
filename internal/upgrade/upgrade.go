package upgrade

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/baggiiiie/configlock/internal/config"
)

const (
	githubReleasesURL = "https://api.github.com/repos/baggiiiie/configlock/releases/latest"
	checkInterval     = 24 * time.Hour // Check once per day
	requestTimeout    = 500 * time.Millisecond
)

// ANSI color codes for muted output
const (
	colorDim   = "\033[2m"
	colorReset = "\033[0m"
)

// githubRelease represents the GitHub API release response
type githubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
}

// CheckForUpgrade checks if a newer version is available on GitHub
// It caches results and only checks once per day
// Never returns errors - silently fails if offline or on any error
func CheckForUpgrade(currentVersion string) {
	// Skip if version is unknown
	if currentVersion == "" || currentVersion == "unknown" {
		return
	}

	// Try to load config for caching
	cfg, err := config.Load()
	if err != nil {
		// No config file, skip upgrade check
		return
	}

	// Check if we should skip based on cache
	if !shouldCheck(cfg) {
		// Use cached result if available
		if cfg.UpgradeLatestVersion != "" && isNewerVersion(cfg.UpgradeLatestVersion, currentVersion) {
			printUpgradeMessage(cfg.UpgradeLatestVersion, currentVersion)
		}
		return
	}

	// Fetch latest version from GitHub
	latestVersion, err := fetchLatestVersion()
	if err != nil {
		// Network error or timeout - skip silently
		return
	}

	// Update cache
	cfg.UpgradeLastCheck = time.Now().Format(time.RFC3339)
	cfg.UpgradeLatestVersion = latestVersion
	_ = cfg.Save() // Ignore save errors

	// Check if upgrade is available
	if isNewerVersion(latestVersion, currentVersion) {
		printUpgradeMessage(latestVersion, currentVersion)
	}
}

// shouldCheck returns true if enough time has passed since the last check
func shouldCheck(cfg *config.Config) bool {
	if cfg.UpgradeLastCheck == "" {
		return true
	}

	lastCheck, err := time.Parse(time.RFC3339, cfg.UpgradeLastCheck)
	if err != nil {
		return true
	}

	return time.Since(lastCheck) >= checkInterval
}

// fetchLatestVersion fetches the latest release version from GitHub
func fetchLatestVersion() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, githubReleasesURL, nil)
	if err != nil {
		return "", err
	}

	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "configlock-upgrade-check")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	return release.TagName, nil
}

// isNewerVersion checks if latestVersion is newer than currentVersion
// Handles versions with or without 'v' prefix
func isNewerVersion(latestVersion, currentVersion string) bool {
	latest := normalizeVersion(latestVersion)
	current := normalizeVersion(currentVersion)

	if latest == "" || current == "" {
		return false
	}

	// Simple comparison: split by dots and compare each part
	latestParts := strings.Split(latest, ".")
	currentParts := strings.Split(current, ".")

	// Pad with zeros if needed
	maxLen := max(len(currentParts), len(latestParts))

	for i := 0; i < maxLen; i++ {
		var latestPart, currentPart int

		if i < len(latestParts) {
			fmt.Sscanf(latestParts[i], "%d", &latestPart)
		}
		if i < len(currentParts) {
			fmt.Sscanf(currentParts[i], "%d", &currentPart)
		}

		if latestPart > currentPart {
			return true
		}
		if latestPart < currentPart {
			return false
		}
	}

	return false // Equal versions
}

// normalizeVersion strips 'v' prefix and any pre-release suffix
func normalizeVersion(v string) string {
	v = strings.TrimPrefix(v, "v")
	// Remove any pre-release suffix (e.g., -beta, -rc1)
	if idx := strings.IndexAny(v, "-+"); idx != -1 {
		v = v[:idx]
	}
	return v
}

// printUpgradeMessage prints the upgrade notification in muted colors
func printUpgradeMessage(latestVersion, currentVersion string) {
	fmt.Fprintf(os.Stderr, "\n%sA new version of configlock is available: %s (current: %s)%s\n",
		colorDim, latestVersion, currentVersion, colorReset)
	fmt.Fprintf(os.Stderr, "%sRun 'brew upgrade configlock' or visit https://github.com/baggiiiie/configlock/releases%s\n",
		colorDim, colorReset)
}
