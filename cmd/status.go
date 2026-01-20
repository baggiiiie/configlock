package cmd

import (
	"fmt"
	"time"

	"github.com/baggiiiie/configlock/internal/config"
	"github.com/baggiiiie/configlock/internal/service"
	kardianos "github.com/kardianos/service"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current status and configuration",
	Long:  `Display the current lock status, lock hours, and active temporary unlocks.`,
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

	fmt.Printf("Lock Hours: %s - %s\n", cfg.StartTime, cfg.EndTime)

	// Current state
	now := time.Now()
	weekday := now.Weekday()
	withinWorkHours := cfg.IsWithinWorkHours()

	fmt.Printf("Current Time: %s\n", now.Format("Monday, 02 Jan 2006 15:04:05"))

	if withinWorkHours {
		fmt.Println("Status: 🔒 LOCKED")
	} else {
		if weekday == time.Saturday || weekday == time.Sunday {
			fmt.Println("Status: 🔓 UNLOCKED")
		} else {
			fmt.Println("Status: 🔓 UNLOCKED")
		}
	}

	// Check daemon status
	svc, err := service.New()
	var daemonRunning bool
	if err == nil {
		status, err := svc.Status()
		if err == nil && status == kardianos.StatusRunning {
			daemonRunning = true
			fmt.Println("Daemon: ✓ Running")
		} else {
			daemonRunning = false
			fmt.Println("Daemon: ✗ Not running")
		}
	} else {
		fmt.Printf("Daemon: ⚠ Unable to check status (%v)\n", err)
	}

	// Show warning if there's a mismatch
	if withinWorkHours && !daemonRunning {
		fmt.Println("\n⚠️  WARNING: During lock hours but daemon not running!")
		fmt.Println("    Locks will not be enforced. Run 'configlock start' to start the daemon.")
	} else if !withinWorkHours && daemonRunning {
		fmt.Println("\n✓ Daemon is running and will enforce locks during lock hours.")
	}

	fmt.Println()

	// Locked paths
	fmt.Printf("Locked Paths: %d\n", len(cfg.LockedPaths))
	if len(cfg.LockedPaths) > 0 {
		fmt.Println("Use 'configlock list' to see all locked paths")
	}
	fmt.Println()

	fmt.Printf("Temp Unlock Duration: %d minutes\n", cfg.TempDuration)
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
