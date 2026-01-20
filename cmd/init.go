package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/baggiiiie/configlock/internal/config"
	"github.com/baggiiiie/configlock/internal/locker"
	"github.com/baggiiiie/configlock/internal/service"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize configlock and install the daemon",
	Long: `Initialize configlock by creating the configuration file,
prompting for lock hours, and installing the daemon as a system service.`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	fmt.Println("Initializing ConfigLock...")

	// Create config directory
	configDir := config.GetConfigDir()
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Check if config already exists
	configPath := config.GetConfigPath()
	if _, err := os.Stat(configPath); err == nil {
		fmt.Print("Config file already exists. Overwrite? (y/N): ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))
		if response != "y" && response != "yes" {
			fmt.Println("Initialization cancelled.")
			return nil
		}
	}

	// Prompt for lock hours configuration
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("\nlock hours configuration:")
	fmt.Println("  - Simple time range: Enter start time (e.g., 0800 or 08:00)")
	fmt.Println("  - Cron schedule: Use 'cron:' prefix (e.g., cron:0 8-17 * * 1-5)")

	var cfg *config.Config
	var startTime, endTime string

	// Get start time or cron schedule with retry
	for {
		fmt.Print("\nlock hours start time or cron schedule (default 08:00): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			input = "08:00"
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

			// Get temp duration before creating config
			fmt.Print("\nTemporary unlock duration in minutes (default 5): ")
			durationStr, _ := reader.ReadString('\n')
			durationStr = strings.TrimSpace(durationStr)
			tempDuration := 5
			if durationStr != "" {
				duration, err := strconv.Atoi(durationStr)
				if err != nil {
					return fmt.Errorf("invalid duration: %w", err)
				}
				tempDuration = duration
			}

			cfg = config.CreateWithCron(cronSchedule, tempDuration)
			fmt.Printf("✓ Using cron schedule: %s\n", cronSchedule)
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
			fmt.Print("lock hours end time (default 17:00): ")
			endInput, _ := reader.ReadString('\n')
			endInput = strings.TrimSpace(endInput)

			if endInput == "" {
				endInput = "17:00"
			}

			normalized, err := config.NormalizeTimeInput(endInput)
			if err != nil {
				fmt.Printf("Error: %v. Please try again.\n", err)
				continue
			}
			endTime = normalized
			break
		}

		// Get temp duration
		fmt.Print("\nTemporary unlock duration in minutes (default 5): ")
		durationStr, _ := reader.ReadString('\n')
		durationStr = strings.TrimSpace(durationStr)
		tempDuration := 5
		if durationStr != "" {
			duration, err := strconv.Atoi(durationStr)
			if err != nil {
				return fmt.Errorf("invalid duration: %w", err)
			}
			tempDuration = duration
		}

		cfg = config.CreateDefault(startTime, endTime, tempDuration)
		fmt.Printf("✓ Using time range: %s - %s (weekdays only)\n", startTime, endTime)
		break
	}

	// Add config file itself to locked paths
	cfg.AddPath(configPath)

	// Save config
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("✓ Config created at %s\n", configPath)

	// Apply lock to config file immediately if within lock hours
	if cfg.IsWithinWorkHours() {
		fmt.Println("Applying lock to config file (within lock hours)...")
		if err := locker.Lock(configPath); err != nil {
			fmt.Printf("Warning: failed to lock config file %s: %v\n", configPath, err)
		} else {
			fmt.Println("✓ Config file locked")
		}
	} else {
		fmt.Println("Note: Outside lock hours. Config file will be locked during lock hours.")
	}

	// Install and start daemon
	fmt.Println("Installing daemon service...")
	svc, err := service.New()
	if err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}

	if err := svc.Install(); err != nil {
		return fmt.Errorf("failed to install service: %w", err)
	}

	fmt.Println("✓ Daemon installed successfully")

	// Start the service
	fmt.Println("Starting daemon...")
	if err := svc.Start(); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	fmt.Println("✓ Daemon started successfully")
	fmt.Println("\nConfigLock is now active!")
	if cfg.CronSchedule != "" {
		fmt.Printf("lock hours: %s (cron schedule)\n", cfg.CronSchedule)
	} else {
		fmt.Printf("lock hours: %s - %s (weekdays only)\n", startTime, endTime)
	}
	fmt.Println("Use 'configlock add <path>' to add files/directories to lock.")

	return nil
}
