package cmd

import (
	"fmt"

	"github.com/baggiiiie/configlock/internal/challenge"
	"github.com/baggiiiie/configlock/internal/config"
	"github.com/baggiiiie/configlock/internal/locker"
	"github.com/baggiiiie/configlock/internal/service"
	"github.com/spf13/cobra"
)

var stopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop configlock and unlock all files",
	Long: `Stop the configlock daemon and unlock all currently locked files.
This requires completing a typing challenge to prevent impulsive actions.

This will:
  - Stop the configlock daemon service
  - Unlock all currently locked files and directories
  - Revert all immutable flags that were applied

To re-enable configlock later, use 'configlock start' to restart the daemon.`,
	RunE: runStop,
}

func init() {
	rootCmd.AddCommand(stopCmd)
}

func runStop(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if len(cfg.LockedPaths) == 0 {
		fmt.Println("No paths are currently locked.")
		fmt.Println("\nChecking daemon status...")

		// Still try to stop the daemon
		svc, err := service.New()
		if err != nil {
			fmt.Printf("Warning: failed to create service: %v\n", err)
			return nil
		}

		if err := svc.Stop(); err != nil {
			fmt.Printf("Daemon is already stopped or not installed.\n")
		} else {
			fmt.Println("✓ Daemon stopped")
		}

		return nil
	}

	fmt.Printf("This will unlock %d path(s) and stop the configlock daemon.\n\n", len(cfg.LockedPaths))
	fmt.Println("Locked paths:")
	for _, path := range cfg.LockedPaths {
		fmt.Printf("  - %s\n", path)
	}
	fmt.Println()

	// Run typing challenge
	if err := challenge.Run(); err != nil {
		return fmt.Errorf("challenge failed: %w", err)
	}

	fmt.Println()

	// Stop the daemon first
	fmt.Println("Stopping daemon...")
	svc, err := service.New()
	if err != nil {
		fmt.Printf("Warning: failed to create service: %v\n", err)
	} else {
		if err := svc.Stop(); err != nil {
			fmt.Printf("Warning: failed to stop service: %v\n", err)
		} else {
			fmt.Println("✓ Daemon stopped")
		}
	}

	// Unlock all paths
	fmt.Println("\nUnlocking all paths...")
	var unlockErrors []string
	successCount := 0

	for _, path := range cfg.LockedPaths {
		fmt.Printf("  Unlocking: %s\n", path)
		if err := locker.Unlock(path); err != nil {
			errMsg := fmt.Sprintf("    Warning: failed to unlock %s: %v", path, err)
			fmt.Println(errMsg)
			unlockErrors = append(unlockErrors, errMsg)
		} else {
			fmt.Printf("    ✓ Unlocked: %s\n", path)
			successCount++
		}
	}

	fmt.Println()

	// Summary
	if len(unlockErrors) > 0 {
		fmt.Printf("✓ Successfully unlocked %d/%d path(s)\n", successCount, len(cfg.LockedPaths))
		fmt.Printf("⚠ %d path(s) had unlock errors. You may need to manually unlock them.\n", len(unlockErrors))
	} else {
		fmt.Printf("✓ All %d path(s) unlocked successfully\n", len(cfg.LockedPaths))
	}

	fmt.Println("\nConfigLock has been stopped.")
	fmt.Println("To re-enable:")
	fmt.Println("  - The locked paths are still saved in your config")
	fmt.Println("  - Run 'configlock start' to restart the daemon")
	fmt.Println("  - Locks will be re-applied during work hours")

	return nil
}
