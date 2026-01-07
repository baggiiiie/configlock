package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/baggiiiie/configlock/internal/config"
	"github.com/baggiiiie/configlock/internal/daemon"
	"github.com/baggiiiie/configlock/internal/locker"
	"github.com/spf13/cobra"
)

var noBackup bool

var addCmd = &cobra.Command{
	Use:   "add <path>",
	Short: "Add a file or directory to the lock list",
	Long: `Add a file or directory to the lock list. If a directory is specified,
all files in the directory (excluding .git/ and .jj/) will be added recursively.`,
	Args: cobra.ExactArgs(1),
	RunE: runAdd,
}

func init() {
	rootCmd.AddCommand(addCmd)
	addCmd.Flags().BoolVar(&noBackup, "no-backup", false, "Skip creating .bak backup files")
}

func runAdd(cmd *cobra.Command, args []string) error {
	path := args[0]

	// Resolve to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Check if path exists
	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("path does not exist: %s", absPath)
	}

	// Load config
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	var pathsToAdd []string

	if info.IsDir() {
		// Collect all files in directory recursively
		fmt.Printf("Collecting files in directory %s...\n", absPath)
		files, err := daemon.CollectFilesRecursively(absPath)
		if err != nil {
			return fmt.Errorf("failed to collect files: %w", err)
		}
		pathsToAdd = files
		fmt.Printf("Found %d files\n", len(files))
	} else {
		pathsToAdd = []string{absPath}
	}

	// Create backups if requested
	if !noBackup {
		fmt.Println("Creating backups...")
		for _, p := range pathsToAdd {
			backupPath := p + ".bak"
			if err := copyFile(p, backupPath); err != nil {
				fmt.Printf("Warning: failed to backup %s: %v\n", p, err)
			}
		}
	}

	// Add paths to config
	for _, p := range pathsToAdd {
		cfg.AddPath(p)
	}

	// Save config
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("✓ Added %d path(s) to lock list\n", len(pathsToAdd))

	// Apply locks immediately if within work hours
	if cfg.IsWithinWorkHours() {
		fmt.Println("Applying locks (within work hours)...")
		for _, p := range pathsToAdd {
			if err := locker.Lock(p); err != nil {
				fmt.Printf("Warning: failed to lock %s: %v\n", p, err)
			}
		}
		fmt.Println("✓ Locks applied")
	} else {
		fmt.Println("Note: Outside work hours. Locks will be applied during work hours.")
	}

	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	return os.WriteFile(dst, data, info.Mode())
}
