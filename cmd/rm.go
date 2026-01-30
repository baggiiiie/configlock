package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/baggiiiie/configlock/internal/challenge"
	"github.com/baggiiiie/configlock/internal/config"
	"github.com/baggiiiie/configlock/internal/locker"
	"github.com/baggiiiie/configlock/internal/service"
	kardianos "github.com/kardianos/service"
	"github.com/spf13/cobra"
)

var rmCmd = &cobra.Command{
	Use:   "rm <path>",
	Short: "Remove a file or directory from the lock list",
	Long: `Remove a file or directory from the lock list. This requires
completing a typing challenge to prevent impulsive actions.`,
	Args: cobra.ExactArgs(1),
	RunE: runRm,
}

func init() {
	rootCmd.AddCommand(rmCmd)
}

func runRm(cmd *cobra.Command, args []string) error {
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
	found := slices.Contains(cfg.LockedPaths, absPath)

	if !found {
		return fmt.Errorf("path not found in lock list: %s", absPath)
	}

	// Run typing challenge only during lock hours
	if cfg.IsWithinWorkHours() {
		if err := challenge.Require("challenge failed"); err != nil {
			return err
		}
	}

	// Remove path from config
	cfg.RemovePath(absPath)

	// Save config
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Check if it's a file or directory for display purposes
	info, err := os.Stat(absPath)
	if err == nil && info.IsDir() {
		fmt.Printf("✓ Removed directory from lock list: %s\n", absPath)
	} else {
		fmt.Printf("✓ Removed file from lock list: %s\n", absPath)
	}

	// Unlock the path immediately (locker will handle directories recursively)
	fmt.Println("Unlocking path...")
	if err := locker.Unlock(absPath); err != nil {
		fmt.Printf("Warning: failed to unlock %s: %v\n", absPath, err)
	} else {
		fmt.Println("✓ Path unlocked")
	}

	// Restart daemon if running to pick up configuration changes
	svc, err := service.New()
	if err == nil {
		status, err := svc.Status()
		if err == nil && status == kardianos.StatusRunning {
			fmt.Println("\nRestarting daemon to apply configuration changes...")
			if err := svc.Restart(); err != nil {
				// Restart might not be supported, try stop+start
				if err := svc.Stop(); err == nil {
					if err := svc.Start(); err != nil {
						fmt.Printf("Warning: failed to restart daemon: %v\n", err)
						return nil
					}
				}
			}
			fmt.Println("✓ Daemon restarted")
		}
	}

	return nil
}
