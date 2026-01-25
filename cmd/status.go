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

	fmt.Printf("Lock Hours: %s - %s (Days: %s)\n", cfg.StartTime, cfg.EndTime, config.FormatDays(cfg.LockDays))

	withinWorkHours := cfg.IsWithinWorkHours()

	// Check daemon status
	svc, err := service.New()
	if err != nil {
		fmt.Printf("Daemon: ⚠ Unable to check (%v)\n", err)
	}
	status, _ := svc.Status()
	daemonRunning := status == kardianos.StatusRunning

	if daemonRunning {
		if withinWorkHours {
			fmt.Println("Status: Locks enforced")
		} else {
			fmt.Println("Status: Daemon idle until lock hours")
		}
	} else {
		if withinWorkHours {
			fmt.Println("Status: Daemon not running! Run 'configlock start'")
		} else {
			fmt.Println("Status: Daemon not running")
		}
	}

	fmt.Println()

	// Locked paths
	fmt.Printf("Locked Paths: %d\n", len(cfg.LockedPaths))
	if len(cfg.LockedPaths) > 0 {
		fmt.Println("- Use 'configlock list' to see all locked paths")
	}
	// Temporary exclusions
	cfg.CleanExpiredExcludes()
	if len(cfg.TempExcludes) > 0 {
		fmt.Printf("Active Temporary Unlocks: %d\n", len(cfg.TempExcludes))
		for path, expiryStr := range cfg.TempExcludes {
			if expiry, err := time.Parse(time.RFC3339, expiryStr); err == nil {
				fmt.Printf("  - %s (expires in %s)\n", path, formatDuration(time.Until(expiry)))
			}
		}
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
