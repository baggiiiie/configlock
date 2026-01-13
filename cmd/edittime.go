package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/baggiiiie/configlock/internal/config"
	"github.com/baggiiiie/configlock/internal/service"
	kardianos "github.com/kardianos/service"
	"github.com/spf13/cobra"
)

var editTimeCmd = &cobra.Command{
	Use:   "edit time",
	Short: "Edit work hours configuration",
	Long: `Edit the work hours configuration for ConfigLock.

This allows you to change between simple time range mode and cron schedule mode,
or update the existing time settings. If the daemon is running, it will be
automatically restarted to apply the changes immediately.`,
	RunE: runEditTime,
}

func init() {
	rootCmd.AddCommand(editTimeCmd)
}

func runEditTime(cmd *cobra.Command, args []string) error {
	// Load existing config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Show current configuration
	fmt.Println("Current work hours configuration:")
	if cfg.CronSchedule != "" {
		fmt.Printf("  Mode: Cron schedule\n")
		fmt.Printf("  Schedule: %s\n", cfg.CronSchedule)
	} else {
		fmt.Printf("  Mode: Simple time range\n")
		fmt.Printf("  Start time: %s\n", cfg.StartTime)
		fmt.Printf("  End time: %s\n", cfg.EndTime)
	}
	fmt.Println()

	// Prompt for new configuration
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("\nNew work hours configuration:")
	fmt.Println("  - Simple time range: Enter start time (e.g., 0800 or 08:00)")
	fmt.Println("  - Cron schedule: Use 'cron:' prefix (e.g., cron:0 8-17 * * 1-5)")

	var startTime, endTime string

	// Get start time or cron schedule with retry
	for {
		fmt.Print("\nWork hours start time or cron schedule (press Enter to keep current): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		// If empty, keep current configuration
		if input == "" {
			fmt.Println("✓ Keeping current work hours configuration")
			break
		}

		// Check if it's a cron expression
		if strings.HasPrefix(input, "cron:") {
			cronSchedule := strings.TrimPrefix(input, "cron:")
			cronSchedule = strings.TrimSpace(cronSchedule)

			if cronSchedule == "" {
				fmt.Println("Error: cron schedule cannot be empty. Please try again.")
				continue
			}

			// Validate cron schedule
			if err := config.ValidateCronSchedule(cronSchedule); err != nil {
				fmt.Printf("Error: %v. Please try again.\n", err)
				continue
			}

			// Update config
			cfg.CronSchedule = cronSchedule
			cfg.StartTime = ""
			cfg.EndTime = ""

			fmt.Printf("✓ Updated to cron schedule: %s\n", cronSchedule)
			break
		}

		// Try to parse as time
		normalized, err := config.NormalizeTimeInput(input)
		if err != nil {
			fmt.Printf("Error: %v. Please try again.\n", err)
			continue
		}
		startTime = normalized

		// Get end time with retry
		for {
			fmt.Print("Work hours end time: ")
			endInput, _ := reader.ReadString('\n')
			endInput = strings.TrimSpace(endInput)

			if endInput == "" {
				fmt.Println("Error: end time cannot be empty. Please try again.")
				continue
			}

			normalized, err := config.NormalizeTimeInput(endInput)
			if err != nil {
				fmt.Printf("Error: %v. Please try again.\n", err)
				continue
			}
			endTime = normalized
			break
		}

		// Update config
		cfg.StartTime = startTime
		cfg.EndTime = endTime
		cfg.CronSchedule = ""

		fmt.Printf("✓ Updated to simple time range: %s - %s (weekdays only)\n", startTime, endTime)
		break
	}

	// Prompt for temp duration update
	fmt.Printf("\nCurrent temporary unlock duration: %d minutes\n", cfg.TempDuration)
	fmt.Print("Update temporary unlock duration in minutes (press Enter to keep current): ")
	durationStr, _ := reader.ReadString('\n')
	durationStr = strings.TrimSpace(durationStr)
	if durationStr != "" {
		duration, err := strconv.Atoi(durationStr)
		if err != nil {
			return fmt.Errorf("invalid duration: %w", err)
		}
		cfg.TempDuration = duration
		fmt.Printf("✓ Updated temporary unlock duration to %d minutes\n", duration)
	}

	// Save updated config
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println("\n✓ Configuration updated successfully!")

	// Automatically restart daemon if it's running
	svc, err := service.New()
	if err != nil {
		fmt.Printf("\nWarning: failed to create service: %v\n", err)
		fmt.Println("\nTo apply the changes manually, restart the daemon:")
		fmt.Println("  configlock stop")
		fmt.Println("  configlock start")
		return nil
	}

	// Check if daemon is running
	status, err := svc.Status()
	if err != nil || status != kardianos.StatusRunning {
		fmt.Println("\nDaemon is not running. Changes will take effect when you start it:")
		fmt.Println("  configlock start")
		return nil
	}

	// Restart the daemon to apply changes
	fmt.Println("\nRestarting daemon to apply changes...")
	if err := svc.Restart(); err != nil {
		// Restart might not be supported on all platforms, try stop+start
		fmt.Println("Restart not supported, stopping and starting daemon...")
		if err := svc.Stop(); err != nil {
			fmt.Printf("Warning: failed to stop daemon: %v\n", err)
		}
		if err := svc.Start(); err != nil {
			return fmt.Errorf("failed to start daemon: %w", err)
		}
	}

	fmt.Println("✓ Daemon restarted successfully")
	fmt.Println("\nYour configuration changes are now active!")

	return nil
}
