package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/baggiiiie/configlock/internal/config"
	"github.com/baggiiiie/configlock/internal/service"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize configlock and install the daemon",
	Long: `Initialize configlock by creating the configuration file,
prompting for work hours, and installing the daemon as a system service.
This command must be run with sudo.`,
	RunE: runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

func runInit(cmd *cobra.Command, args []string) error {
	// Check if running as root
	if os.Getuid() != 0 {
		return fmt.Errorf("this command must be run with sudo")
	}

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

	// Prompt for work hours
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Work start time (HH:MM, default 08:00): ")
	startTime, _ := reader.ReadString('\n')
	startTime = strings.TrimSpace(startTime)
	if startTime == "" {
		startTime = "08:00"
	}

	fmt.Print("Work end time (HH:MM, default 17:00): ")
	endTime, _ := reader.ReadString('\n')
	endTime = strings.TrimSpace(endTime)
	if endTime == "" {
		endTime = "17:00"
	}

	fmt.Print("Temporary unlock duration in minutes (default 5): ")
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

	// Create config
	cfg := config.CreateDefault(startTime, endTime, tempDuration)

	// Add config file itself to locked paths
	cfg.AddPath(configPath)

	// Save config
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("✓ Config created at %s\n", configPath)

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
	fmt.Printf("Work hours: %s - %s (weekdays only)\n", startTime, endTime)
	fmt.Println("Use 'configlock add <path>' to add files/directories to lock.")

	return nil
}
