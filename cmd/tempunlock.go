package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/baggiiiie/configlock/internal/challenge"
	"github.com/baggiiiie/configlock/internal/config"
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
	if err := challenge.Require("challenge failed"); err != nil {
		return err
	}

	// Add temporary exclusion for the path
	cfg.AddTempExclude(absPath, unlockDuration)

	// Save config
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Unlock the path immediately (locker will handle directories recursively)
	fmt.Println("Unlocking path...")
	if err := locker.Unlock(absPath); err != nil {
		fmt.Printf("Warning: failed to unlock %s: %v\n", absPath, err)
	}

	// Check if it's a file or directory for display purposes
	info, err := os.Stat(absPath)
	if err == nil && info.IsDir() {
		fmt.Printf("✓ Temporarily unlocked directory for %d minutes: %s\n", unlockDuration, absPath)
	} else {
		fmt.Printf("✓ Temporarily unlocked file for %d minutes: %s\n", unlockDuration, absPath)
	}

	return nil
}
