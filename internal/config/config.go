package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/baggiiiie/configlock/internal/locker"
)

// Config represents the configlock configuration
type Config struct {
	LockedPaths  []string          `json:"locked_paths"`
	StartTime    string            `json:"start_time"`    // "08:00"
	EndTime      string            `json:"end_time"`      // "17:00"
	LockDays     []int             `json:"lock_days"`     // e.g., [1, 2, 3, 4, 5] for weekdays
	TempDuration int               `json:"temp_duration"` // minutes
	TempExcludes map[string]string `json:"temp_excludes"` // path -> expiration ISO8601

	// Upgrade check cache
	UpgradeLastCheck     string `json:"upgrade_last_check,omitempty"`     // ISO8601 timestamp
	UpgradeLatestVersion string `json:"upgrade_latest_version,omitempty"` // cached latest version

	mu sync.RWMutex `json:"-"`
}

var (
	configPath string
	configDir  string
)

func init() {
	home, err := os.UserHomeDir()
	if err != nil {
		panic(fmt.Sprintf("failed to get home directory: %v", err))
	}
	configDir = filepath.Join(home, ".config", "configlock")
	configPath = filepath.Join(configDir, "config.json")
}

// GetConfigPath returns the path to the config file
func GetConfigPath() string {
	return configPath
}

// GetConfigDir returns the path to the config directory
func GetConfigDir() string {
	return configDir
}

// Load reads and parses the config file
func Load() (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("config file not found. Please run 'configlock init' first to initialize")
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if cfg.TempExcludes == nil {
		cfg.TempExcludes = make(map[string]string)
	}

	return &cfg, nil
}

// Save writes the config to disk with file locking
func (c *Config) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Unlock config file before writing (if it's locked)
	// This allows configlock to modify its own config file even when locked
	wasLocked := false
	if locked, err := locker.IsLocked(configPath); err == nil && locked {
		wasLocked = true
		if err := locker.Unlock(configPath); err != nil {
			return fmt.Errorf("failed to unlock config for writing: %w", err)
		}
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write atomically using a temp file
	tmpPath := configPath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o600); err != nil {
		return fmt.Errorf("failed to write temp config: %w", err)
	}

	if err := os.Rename(tmpPath, configPath); err != nil {
		return fmt.Errorf("failed to rename config: %w", err)
	}

	// Re-lock config file after writing (if it was locked before)
	if wasLocked {
		if err := locker.Lock(configPath); err != nil {
			return fmt.Errorf("failed to re-lock config after writing: %w", err)
		}
	}

	return nil
}

// AddPath adds a path to the locked paths list (deduplicates)
func (c *Config) AddPath(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if already exists
	if slices.Contains(c.LockedPaths, path) {
		return
	}

	c.LockedPaths = append(c.LockedPaths, path)
}

// RemovePath removes a path from the locked paths list
func (c *Config) RemovePath(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	newPaths := make([]string, 0, len(c.LockedPaths))
	for _, p := range c.LockedPaths {
		if p != path {
			newPaths = append(newPaths, p)
		}
	}
	c.LockedPaths = newPaths
}

// AddTempExclude adds a temporary exclusion with expiration
func (c *Config) AddTempExclude(path string, duration int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	expiration := time.Now().Add(time.Duration(duration) * time.Minute)
	c.TempExcludes[path] = expiration.Format(time.RFC3339)
}

// RemoveTempExclude removes a temporary exclusion
func (c *Config) RemoveTempExclude(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.TempExcludes, path)
}

// CleanExpiredExcludes removes expired temporary exclusions
// Returns true if any exclusions were removed
func (c *Config) CleanExpiredExcludes() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	cleaned := false
	now := time.Now()
	for path, expiryStr := range c.TempExcludes {
		expiry, err := time.Parse(time.RFC3339, expiryStr)
		if err != nil || expiry.Before(now) {
			delete(c.TempExcludes, path)
			cleaned = true
		}
	}
	return cleaned
}

// IsTemporarilyExcluded checks if a path is temporarily excluded
func (c *Config) IsTemporarilyExcluded(path string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	expiryStr, exists := c.TempExcludes[path]
	if !exists {
		return false
	}

	expiry, err := time.Parse(time.RFC3339, expiryStr)
	if err != nil {
		return false
	}

	return expiry.After(time.Now())
}

// IsWithinWorkHours checks if the current time is within lock hours
func (c *Config) IsWithinWorkHours() bool {
	now := time.Now()
	weekday := int(now.Weekday())
	if weekday == 0 { // Sunday
		weekday = 7
	}

	isLockDay := slices.Contains(c.LockDays, weekday)
	if !isLockDay {
		return false
	}

	// Parse start and end times
	startTime, err := time.Parse("15:04", c.StartTime)
	if err != nil {
		return false
	}

	endTime, err := time.Parse("15:04", c.EndTime)
	if err != nil {
		return false
	}

	// Get current time in HH:MM format
	currentTime := time.Date(0, 1, 1, now.Hour(), now.Minute(), 0, 0, time.UTC)
	start := time.Date(0, 1, 1, startTime.Hour(), startTime.Minute(), 0, 0, time.UTC)
	end := time.Date(0, 1, 1, endTime.Hour(), endTime.Minute(), 0, 0, time.UTC)

	return (currentTime.Equal(start) || currentTime.After(start)) && currentTime.Before(end)
}

// TimeUntilWorkHours returns the duration until work hours start
// Returns 0 if already within work hours
func (c *Config) TimeUntilWorkHours() time.Duration {
	now := time.Now()

	if c.IsWithinWorkHours() {
		return 0
	}

	// Parse start time
	startTime, err := time.Parse("15:04", c.StartTime)
	if err != nil {
		return time.Hour // fallback to 1 hour
	}

	// Build next start time today
	nextStart := time.Date(now.Year(), now.Month(), now.Day(),
		startTime.Hour(), startTime.Minute(), 0, 0, now.Location())

	for range 8 {
		weekday := int(nextStart.Weekday())
		if weekday == 0 { // Sunday
			weekday = 7
		}

		isLockDay := slices.Contains(c.LockDays, weekday)

		if isLockDay && now.Before(nextStart) {
			return nextStart.Sub(now)
		}

		// Move to next day
		nextStart = nextStart.Add(24 * time.Hour)
	}

	return time.Hour // fallback
}

// CreateDefault creates a new config with default values
func CreateDefault(startTime, endTime string, lockDays []int, tempDuration int) *Config {
	return &Config{
		LockedPaths:  []string{},
		StartTime:    startTime,
		EndTime:      endTime,
		LockDays:     lockDays,
		TempDuration: tempDuration,
		TempExcludes: make(map[string]string),
	}
}

// NormalizeTimeRange parses a time range string and returns start and end times
func NormalizeTimeRange(input string) (string, string, error) {
	parts := strings.Split(input, "-")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid time range format: %s (expected HHMM-HHMM or H-H)", input)
	}

	startTime, err := normalizeTime(parts[0])
	if err != nil {
		return "", "", err
	}
	endTime, err := normalizeTime(parts[1])
	if err != nil {
		return "", "", err
	}
	return startTime, endTime, nil
}

func normalizeTime(input string) (string, error) {
	input = strings.TrimSpace(input)

	if strings.Contains(input, ":") {
		_, err := time.Parse("15:04", input)
		if err != nil {
			return "", fmt.Errorf("invalid time format: %s", input)
		}
		return input, nil
	}

	re := regexp.MustCompile(`\D`)
	digits := re.ReplaceAllString(input, "")

	var hour, minute string
	switch len(digits) {
	case 1: // H
		hour = "0" + digits
		minute = "00"
	case 2: // HH
		hour = digits
		minute = "00"
	case 3: // HMM
		hour = "0" + digits[:1]
		minute = digits[1:]
	case 4: // HHMM
		hour = digits[:2]
		minute = digits[2:]
	default:
		return "", fmt.Errorf("invalid time format: %s", input)
	}

	normalized := fmt.Sprintf("%s:%s", hour, minute)
	_, err := time.Parse("15:04", normalized)
	if err != nil {
		return "", fmt.Errorf("invalid time: %s", input)
	}
	return normalized, nil
}

// ParseDays parses a day range string (e.g., "1-5" or "1,2,5") into a slice of integers
func ParseDays(input string) ([]int, error) {
	var days []int
	parts := strings.SplitSeq(input, ",")
	for part := range parts {
		part = strings.TrimSpace(part)
		if strings.Contains(part, "-") {
			rangeParts := strings.Split(part, "-")
			if len(rangeParts) != 2 {
				return nil, fmt.Errorf("invalid day range: %s", part)
			}
			start, err := strconv.Atoi(rangeParts[0])
			if err != nil {
				return nil, fmt.Errorf("invalid day: %s", rangeParts[0])
			}
			end, err := strconv.Atoi(rangeParts[1])
			if err != nil {
				return nil, fmt.Errorf("invalid day: %s", rangeParts[1])
			}
			if start < 1 || end > 7 || start > end {
				return nil, fmt.Errorf("invalid day range: %d-%d", start, end)
			}
			for i := start; i <= end; i++ {
				days = append(days, i)
			}
		} else {
			day, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("invalid day: %s", part)
			}
			if day < 1 || day > 7 {
				return nil, fmt.Errorf("invalid day: %d (must be 1-7)", day)
			}
			days = append(days, day)
		}
	}
	// Deduplicate
	daySet := make(map[int]struct{})
	var uniqueDays []int
	for _, day := range days {
		if _, exists := daySet[day]; !exists {
			daySet[day] = struct{}{}
			uniqueDays = append(uniqueDays, day)
		}
	}
	return uniqueDays, nil
}

// FormatDays formats a slice of days into a human-readable string
func FormatDays(days []int) string {
	dayMap := map[int]string{
		1: "Mon",
		2: "Tue",
		3: "Wed",
		4: "Thu",
		5: "Fri",
		6: "Sat",
		7: "Sun",
	}

	var parts []string
	// Sort days for consistent output
	sort.Ints(days)
	for _, day := range days {
		if name, ok := dayMap[day]; ok {
			parts = append(parts, name)
		}
	}
	return strings.Join(parts, ", ")
}
