package cmd

import (
	"fmt"
	"time"

	"github.com/baggiiiie/configlock/internal/config"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current status and configuration",
	Long:  `Display the current lock status, work hours, and active temporary unlocks.`,
	RunE:  runStatus,
}

func init() {
	rootCmd.AddCommand(statusCmd)
}

func runStatus(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	fmt.Println("ConfigLock Status")
	fmt.Println("=================")
	fmt.Println()

	// Work hours
	fmt.Printf("Work Hours: %s - %s (weekdays only)\n", cfg.StartTime, cfg.EndTime)
	fmt.Printf("Temp Unlock Duration: %d minutes\n", cfg.TempDuration)
	fmt.Println()

	// Current state
	now := time.Now()
	weekday := now.Weekday()
	withinWorkHours := cfg.IsWithinWorkHours()

	fmt.Printf("Current Time: %s\n", now.Format("Monday, 02 Jan 2006 15:04:05"))
	fmt.Printf("Day: %s\n", weekday)

	if withinWorkHours {
		fmt.Println("Status: 🔒 LOCKED (within work hours)")
	} else {
		if weekday == time.Saturday || weekday == time.Sunday {
			fmt.Println("Status: 🔓 UNLOCKED (weekend)")
		} else {
			fmt.Println("Status: 🔓 UNLOCKED (outside work hours)")
		}
	}
	fmt.Println()

	// Locked paths
	fmt.Printf("Locked Paths: %d\n", len(cfg.LockedPaths))
	if len(cfg.LockedPaths) > 0 {
		fmt.Println("Use 'configlock list' to see all locked paths")
	}
	fmt.Println()

	// Temporary exclusions
	cfg.CleanExpiredExcludes()
	if len(cfg.TempExcludes) > 0 {
		fmt.Printf("Active Temporary Unlocks: %d\n", len(cfg.TempExcludes))
		for path, expiryStr := range cfg.TempExcludes {
			expiry, err := time.Parse(time.RFC3339, expiryStr)
			if err != nil {
				continue
			}
			remaining := time.Until(expiry)
			if remaining > 0 {
				fmt.Printf("  - %s (expires in %s)\n", path, formatDuration(remaining))
			}
		}
	} else {
		fmt.Println("Active Temporary Unlocks: None")
	}

	return nil
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}
