package cmd

import (
	"fmt"

	"github.com/baggiiiie/configlock/internal/service"
	"github.com/spf13/cobra"
)

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the configlock daemon",
	Long: `Start the configlock daemon service to begin enforcing locks during work hours.

This will start the background daemon that monitors and enforces file locks
on all paths in your locked paths list.`,
	RunE: runStart,
}

func init() {
	rootCmd.AddCommand(startCmd)
}

func runStart(cmd *cobra.Command, args []string) error {
	fmt.Println("Starting configlock daemon...")

	svc, err := service.New()
	if err != nil {
		return fmt.Errorf("failed to create service: %w", err)
	}

	if err := svc.Start(); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	fmt.Println("✓ Daemon started successfully")
	fmt.Println("\nConfigLock is now active and will enforce locks during work hours.")

	return nil
}
