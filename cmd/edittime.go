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
	Short: "Edit lock hours configuration",
	Long: `Edit the lock hours configuration for ConfigLock.

This allows you to change the existing time settings. If the daemon is running, it will be
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
	fmt.Println("Current lock hours configuration:")
	fmt.Printf("  Time range: %s - %s\n", cfg.StartTime, cfg.EndTime)
	fmt.Printf("  Lock days: %s\n", config.FormatDays(cfg.LockDays))
	fmt.Println()

	// Prompt for new configuration
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("\nNew lock hours configuration:")
	fmt.Println("  - Time range: Enter a time range like 0800-1700 or 8-17.")
	fmt.Println("  - Day range: Enter a day range like 1-5 (Mon-Fri) or comma-separated days like 1,2,3,4,5.")

	// Get time range with retry
	fmt.Print("\nEnter lock time range (press Enter to keep current): ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input != "" {
		startTime, endTime, err := config.NormalizeTimeRange(input)
		if err != nil {
			return fmt.Errorf("invalid time range: %w", err)
		}
		cfg.StartTime = startTime
		cfg.EndTime = endTime
		fmt.Printf("✓ Updated time range to: %s - %s\n", startTime, endTime)
	} else {
		fmt.Println("✓ Keeping current time range.")
	}

	// Get day range with retry
	fmt.Print("Enter lock days (press Enter to keep current): ")
	input, _ = reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input != "" {
		lockDays, err := config.ParseDays(input)
		if err != nil {
			return fmt.Errorf("invalid day range: %w", err)
		}
		cfg.LockDays = lockDays
		fmt.Printf("✓ Updated lock days to: %s\n", config.FormatDays(lockDays))
	} else {
		fmt.Println("✓ Keeping current lock days.")
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
