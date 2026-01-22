package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/baggiiiie/configlock/internal/locker"
	"github.com/robfig/cron/v3"
)

// Config represents the configlock configuration
type Config struct {
	LockedPaths  []string          `json:"locked_paths"`
	StartTime    string            `json:"start_time"`    // "08:00" (used if CronSchedule is empty)
	EndTime      string            `json:"end_time"`      // "17:00" (used if CronSchedule is empty)
	CronSchedule string            `json:"cron_schedule"` // Optional: cron expression for lock hours (e.g., "0 8-17 * * 1-5" for 8am-5pm weekdays)
	TempDuration int               `json:"temp_duration"` // minutes
	TempExcludes map[string]string `json:"temp_excludes"` // path -> expiration ISO8601
	mu           sync.RWMutex      `json:"-"`
}

var configPath string
var configDir string

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
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
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
	for _, p := range c.LockedPaths {
		if p == path {
			return
		}
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
// If CronSchedule is defined, it uses that; otherwise uses StartTime/EndTime
func (c *Config) IsWithinWorkHours() bool {
	now := time.Now()

	// If cron schedule is defined, use it
	if c.CronSchedule != "" {
		return c.isWithinCronSchedule(now)
	}

	// Otherwise, use simple start/end time logic
	// Check if weekday (Monday = 1, Sunday = 0)
	weekday := now.Weekday()
	if weekday == time.Saturday || weekday == time.Sunday {
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

// isWithinCronSchedule checks if the current time matches the cron schedule
// For lock hours, we interpret the cron schedule as defining when locks should be active
// The schedule should match if the cron would trigger at the current hour
func (c *Config) isWithinCronSchedule(now time.Time) bool {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(c.CronSchedule)
	if err != nil {
		// If parsing fails, log error and return false
		return false
	}

	// Round current time to the start of the current minute
	currentMinute := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), 0, 0, now.Location())

	// Check if the schedule would trigger at the current minute
	// We look for the next scheduled time from one minute ago
	oneMinuteAgo := currentMinute.Add(-1 * time.Minute)
	nextTrigger := schedule.Next(oneMinuteAgo)

	// If the next trigger is at or before the current minute, we're within lock hours
	return !nextTrigger.After(currentMinute)
}

// TimeUntilWorkHours returns the duration until work hours start
// Returns 0 if already within work hours
func (c *Config) TimeUntilWorkHours() time.Duration {
	now := time.Now()

	if c.IsWithinWorkHours() {
		return 0
	}

	// If cron schedule is defined, use it
	if c.CronSchedule != "" {
		return c.timeUntilCronSchedule(now)
	}

	// Parse start time
	startTime, err := time.Parse("15:04", c.StartTime)
	if err != nil {
		return time.Hour // fallback to 1 hour
	}

	// Build next start time today
	nextStart := time.Date(now.Year(), now.Month(), now.Day(),
		startTime.Hour(), startTime.Minute(), 0, 0, now.Location())

	// If we're past today's start time or it's a weekend, find next weekday
	if now.After(nextStart) || now.Weekday() == time.Saturday || now.Weekday() == time.Sunday {
		// Move to next day
		nextStart = nextStart.Add(24 * time.Hour)
		// Skip weekends
		for nextStart.Weekday() == time.Saturday || nextStart.Weekday() == time.Sunday {
			nextStart = nextStart.Add(24 * time.Hour)
		}
	}

	return nextStart.Sub(now)
}

// timeUntilCronSchedule returns duration until the next cron trigger
func (c *Config) timeUntilCronSchedule(now time.Time) time.Duration {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(c.CronSchedule)
	if err != nil {
		return time.Hour // fallback to 1 hour
	}

	nextTrigger := schedule.Next(now)
	return nextTrigger.Sub(now)
}

// ValidateCronSchedule validates a cron schedule expression
func ValidateCronSchedule(cronExpr string) error {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	_, err := parser.Parse(cronExpr)
	if err != nil {
		return fmt.Errorf("invalid cron expression: %w", err)
	}
	return nil
}

// CreateDefault creates a new config with default values
func CreateDefault(startTime, endTime string, tempDuration int) *Config {
	return &Config{
		LockedPaths:  []string{},
		StartTime:    startTime,
		EndTime:      endTime,
		CronSchedule: "", // Empty by default, uses StartTime/EndTime
		TempDuration: tempDuration,
		TempExcludes: make(map[string]string),
	}
}

// CreateWithCron creates a new config with cron schedule
func CreateWithCron(cronSchedule string, tempDuration int) *Config {
	return &Config{
		LockedPaths:  []string{},
		StartTime:    "",
		EndTime:      "",
		CronSchedule: cronSchedule,
		TempDuration: tempDuration,
		TempExcludes: make(map[string]string),
	}
}

// NormalizeTimeInput normalizes time input to "HH:MM" format
// Accepts formats: "HH:MM", "HHMM", "H:MM", "HMM"
// Examples: "08:00", "0800", "8:00", "800" all become "08:00"
func NormalizeTimeInput(input string) (string, error) {
	input = strings.TrimSpace(input)

	// If it already has a colon, validate and return
	if strings.Contains(input, ":") {
		parts := strings.Split(input, ":")
		if len(parts) != 2 {
			return "", fmt.Errorf("invalid time format: %s (expected HH:MM or HHMM)", input)
		}
		// Validate the format
		_, err := time.Parse("15:04", input)
		if err != nil {
			return "", fmt.Errorf("invalid time format: %s (expected HH:MM or HHMM)", input)
		}
		return input, nil
	}

	// Handle formats without colon: HHMM or HMM
	// Remove any non-digit characters
	re := regexp.MustCompile(`\D`)
	digits := re.ReplaceAllString(input, "")

	var hour, minute string
	switch len(digits) {
	case 3:
		// HMM format (e.g., "830" -> "08:30")
		hour = "0" + digits[0:1]
		minute = digits[1:3]
	case 4:
		// HHMM format (e.g., "0830" -> "08:30")
		hour = digits[0:2]
		minute = digits[2:4]
	case 2:
		// HH format (e.g., "08" -> "08:00")
		hour = digits[0:2]
		minute = "00"
	case 1:
		// H format (e.g., "8" -> "08:00")
		hour = "0" + digits[0:1]
		minute = "00"
	default:
		return "", fmt.Errorf("invalid time format: %s (expected HH:MM or HHMM)", input)
	}

	normalized := fmt.Sprintf("%s:%s", hour, minute)

	// Validate the result
	_, err := time.Parse("15:04", normalized)
	if err != nil {
		return "", fmt.Errorf("invalid time: %s (hours must be 00-23, minutes must be 00-59)", input)
	}

	return normalized, nil
}
