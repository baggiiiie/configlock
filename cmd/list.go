package cmd

import (
	"fmt"

	"github.com/baggiiiie/configlock/internal/config"
	"github.com/spf13/cobra"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all locked paths",
	Long:  `Display all files and directories that are currently in the lock list.`,
	RunE:  runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if len(cfg.LockedPaths) == 0 {
		fmt.Println("No paths are currently locked.")
		fmt.Println("Use 'configlock add <path>' to add paths.")
		return nil
	}

	fmt.Printf("Locked Paths (%d):\n", len(cfg.LockedPaths))
	fmt.Println()

	for i, path := range cfg.LockedPaths {
		status := ""
		if cfg.IsTemporarilyExcluded(path) {
			status = " [temporarily unlocked]"
		}
		fmt.Printf("%4d. %s%s\n", i+1, path, status)
	}

	return nil
}
