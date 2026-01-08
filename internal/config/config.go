package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/baggiiiie/configlock/internal/locker"
)

// Config represents the configlock configuration
type Config struct {
	LockedPaths  []string          `json:"locked_paths"`
	StartTime    string            `json:"start_time"`    // "08:00"
	EndTime      string            `json:"end_time"`      // "17:00"
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

// IsWithinWorkHours checks if the current time is within work hours on a weekday
func (c *Config) IsWithinWorkHours() bool {
	now := time.Now()

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

// CreateDefault creates a new config with default values
func CreateDefault(startTime, endTime string, tempDuration int) *Config {
	return &Config{
		LockedPaths:  []string{},
		StartTime:    startTime,
		EndTime:      endTime,
		TempDuration: tempDuration,
		TempExcludes: make(map[string]string),
	}
}
