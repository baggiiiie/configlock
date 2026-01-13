package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/baggiiiie/configlock/internal/config"
	"github.com/baggiiiie/configlock/internal/fileutil"
	"github.com/baggiiiie/configlock/internal/locker"
	"github.com/baggiiiie/configlock/internal/service"
	kardianos "github.com/kardianos/service"
	"github.com/spf13/cobra"
)

var backup bool

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
	addCmd.Flags().BoolVar(&backup, "backup", false, "Create .bak backup files before locking")
}

func runAdd(cmd *cobra.Command, args []string) error {
	path := args[0]

	// Resolve to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Check if path is a symlink and resolve it to the real path
	linfo, err := os.Lstat(absPath)
	if err != nil {
		return fmt.Errorf("failed to check path: %w", err)
	}

	if linfo.Mode()&os.ModeSymlink != 0 {
		realPath, err := filepath.EvalSymlinks(absPath)
		if err != nil {
			return fmt.Errorf("failed to resolve symlink: %w", err)
		}
		fmt.Printf("Resolved symlink %s -> %s\n", absPath, realPath)
		absPath = realPath
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

	// Create backups if requested
	if backup {
		fmt.Println("Creating backups...")
		if info.IsDir() {
			// Collect all files for backup purposes
			files, err := fileutil.CollectFilesRecursively(absPath)
			if err != nil {
				return fmt.Errorf("failed to collect files for backup: %w", err)
			}
			for _, p := range files {
				backupPath := p + ".bak"
				if err := copyFile(p, backupPath); err != nil {
					fmt.Printf("Warning: failed to backup %s: %v\n", p, err)
				}
			}
		} else {
			backupPath := absPath + ".bak"
			if err := copyFile(absPath, backupPath); err != nil {
				fmt.Printf("Warning: failed to backup %s: %v\n", absPath, err)
			}
		}
	}

	// Add path to config (just the directory or file path, not individual files)
	cfg.AddPath(absPath)

	// Save config
	if err := cfg.Save(); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	if info.IsDir() {
		fmt.Printf("✓ Added directory to lock list: %s\n", absPath)
	} else {
		fmt.Printf("✓ Added file to lock list: %s\n", absPath)
	}

	// Apply locks immediately if within work hours
	if cfg.IsWithinWorkHours() {
		fmt.Println("Applying locks (within work hours)...")
		if err := locker.Lock(absPath); err != nil {
			fmt.Printf("Warning: failed to lock %s: %v\n", absPath, err)
		} else {
			fmt.Println("✓ Locks applied")
		}
	} else {
		fmt.Println("Note: Outside work hours. Locks will be applied during work hours.")
	}

	// Restart daemon if running to pick up new path
	svc, err := service.New()
	if err == nil {
		status, err := svc.Status()
		if err == nil && status == kardianos.StatusRunning {
			fmt.Println("\nRestarting daemon to apply configuration changes...")
			if err := svc.Restart(); err != nil {
				// Restart might not be supported, try stop+start
				if err := svc.Stop(); err == nil {
					if err := svc.Start(); err != nil {
						fmt.Printf("Warning: failed to restart daemon: %v\n", err)
						return nil
					}
				}
			}
			fmt.Println("✓ Daemon restarted")
		}
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
