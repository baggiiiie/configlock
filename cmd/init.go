package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/baggiiiie/configlock/internal/challenge"
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
	var existingLockedPaths []string
	if _, err := os.Stat(configPath); err == nil {
		// Config exists - check if it's locked
		isLocked, err := locker.IsLocked(configPath)
		if err != nil {
			fmt.Printf("Warning: failed to check if config is locked: %v\n", err)
		}

		if isLocked {
			// Config is locked - require typing challenge to prevent bypass
			fmt.Println("\n⚠️  Config file is currently locked.")
			fmt.Println("Re-initializing will modify the configuration.")
			fmt.Println("You must complete the typing challenge to proceed.")
			fmt.Println()

			if err := challenge.Require("typing challenge failed"); err != nil {
				return err
			}
		} else {
			// Config exists but not locked - just ask for confirmation
			fmt.Print("Config file already exists. Overwrite? (y/N): ")
			reader := bufio.NewReader(os.Stdin)
			response, _ := reader.ReadString('\n')
			response = strings.TrimSpace(strings.ToLower(response))
			if response != "y" && response != "yes" {
				fmt.Println("Initialization cancelled.")
				return nil
			}
		}

		// Load existing config to preserve locked paths
		existingCfg, err := config.Load()
		if err == nil {
			existingLockedPaths = existingCfg.LockedPaths
			fmt.Printf("Preserving %d existing locked path(s)\n", len(existingLockedPaths))
		}
	}

	// Prompt for lock hours configuration
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("\nLock hours configuration:")
	fmt.Println("  - Time range: Enter a time range like 0800-1700 or 8-17.")
	fmt.Println("  - Day range: Enter a day range like 1-5 (Mon-Fri) or comma-separated days like 1,2,3,4,5.")

	var cfg *config.Config
	var startTime, endTime string
	var lockDays []int

	// Get time range with retry
	for {
		fmt.Print("\nEnter lock time range (default 08:00-17:00): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			input = "08:00-17:00"
		}

		var err error
		startTime, endTime, err = config.NormalizeTimeRange(input)
		if err != nil {
			fmt.Printf("Error: %v. Please try again.\n", err)
			continue
		}
		break
	}

	// Get day range with retry
	for {
		fmt.Print("Enter lock days (default 1-5): ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			input = "1-5"
		}

		var err error
		lockDays, err = config.ParseDays(input)
		if err != nil {
			fmt.Printf("Error: %v. Please try again.\n", err)
			continue
		}
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

	cfg = config.CreateDefault(startTime, endTime, lockDays, tempDuration)
	fmt.Printf("✓ Using time range: %s - %s on days: %s\n", startTime, endTime, config.FormatDays(lockDays))

	// Add config file itself to locked paths
	cfg.AddPath(configPath)

	// Restore existing locked paths (if re-initializing)
	if len(existingLockedPaths) > 0 {
		fmt.Println("Restoring existing locked paths...")
		for _, path := range existingLockedPaths {
			// Skip the config path since we already added it
			if path != configPath {
				cfg.AddPath(path)
			}
		}
	}

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
	fmt.Printf("Lock hours: %s - %s on days: %s\n", startTime, endTime, config.FormatDays(lockDays))
	fmt.Println("Use 'configlock add <path>' to add files/directories to lock.")

	return nil
}
