package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/baggiiiie/configlock/internal/challenge"
	"github.com/baggiiiie/configlock/internal/config"
	"github.com/baggiiiie/configlock/internal/daemon"
	"github.com/baggiiiie/configlock/internal/locker"
	"github.com/spf13/cobra"
)

var duration int

var tempUnlockCmd = &cobra.Command{
	Use:   "temp-unlock <path>",
	Short: "Temporarily unlock a file or directory",
	Long: `Temporarily unlock a file or directory for a specified duration.
This requires completing a typing challenge to prevent impulsive actions.`,
	Args: cobra.ExactArgs(1),
	RunE: runTempUnlock,
}

func init() {
	rootCmd.AddCommand(tempUnlockCmd)
	tempUnlockCmd.Flags().IntVar(&duration, "duration", 0, "Duration in minutes (0 = use config default)")
}

func runTempUnlock(cmd *cobra.Command, args []string) error {
	path := args[0]

	// Resolve to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Check if path exists in config
	found := false
	for _, p := range cfg.LockedPaths {
		if p == absPath {
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("path not found in lock list: %s", absPath)
	}

	// Use config default if duration not specified
	unlockDuration := duration
	if unlockDuration == 0 {
		unlockDuration = cfg.TempDuration
	}

	// Run typing challenge
	if err := challenge.Run(); err != nil {
		return fmt.Errorf("challenge failed: %w", err)
	}

	// Determine paths to unlock
	var pathsToUnlock []string

	// Check if it's a directory (for recursive unlocking)
	info, err := os.Stat(absPath)
	if err == nil && info.IsDir() {
		// Collect all files that should be unlocked
		files, err := daemon.CollectFilesRecursively(absPath)
		if err != nil {
			fmt.Printf("Warning: failed to collect files: %v\n", err)
			pathsToUnlock = []string{absPath}
		} else {
			pathsToUnlock = files
		}
	} else {
		pathsToUnlock = []string{absPath}
	}

	// Add temporary exclusions for all paths
	for _, p := range pathsToUnlock {
		cfg.AddTempExclude(p, unlockDuration)
	}

	// Save config
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Unlock the paths immediately
	fmt.Println("Unlocking paths...")
	for _, p := range pathsToUnlock {
		if err := locker.Unlock(p); err != nil {
			fmt.Printf("Warning: failed to unlock %s: %v\n", p, err)
		}
	}

	fmt.Printf("✓ Temporarily unlocked %d path(s) for %d minutes\n", len(pathsToUnlock), unlockDuration)

	return nil
}
