package daemon

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/baggiiiie/configlock/internal/config"
	"github.com/baggiiiie/configlock/internal/locker"
	"github.com/baggiiiie/configlock/internal/logger"
	"github.com/fsnotify/fsnotify"
)

type Daemon struct {
	cfg     *config.Config
	watcher *fsnotify.Watcher
	logger  *logger.Logger
	stopCh  chan struct{}
}

// New creates a new daemon instance
func New() (*Daemon, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	return &Daemon{
		cfg:     cfg,
		watcher: watcher,
		logger:  logger.GetLogger(),
		stopCh:  make(chan struct{}),
	}, nil
}

// Start starts the daemon
func (d *Daemon) Start() error {
	d.logger.Info("Starting configlock daemon")

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

	// Set up file watchers
	if err := d.setupWatchers(); err != nil {
		d.logger.Errorf("Failed to setup watchers: %v", err)
		// Continue anyway, periodic enforcement will still work
	}

	// Start periodic enforcement
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Initial enforcement
	d.enforce()

	for {
		select {
		case <-d.stopCh:
			d.logger.Info("Daemon stopped")
			return nil

		case sig := <-sigCh:
			d.logger.Infof("Received signal: %v", sig)
			if sig == syscall.SIGHUP {
				// Reload config
				d.logger.Info("Reloading configuration")
				cfg, err := config.Load()
				if err != nil {
					d.logger.Errorf("Failed to reload config: %v", err)
				} else {
					d.cfg = cfg
					d.setupWatchers()
				}
			} else {
				// SIGTERM or SIGINT
				d.Stop()
				return nil
			}

		case event := <-d.watcher.Events:
			d.logger.Infof("File event detected: %s %s", event.Op, event.Name)
			// Re-apply locks immediately on any file change
			if d.cfg.IsWithinWorkHours() {
				d.lockPath(event.Name)
			}

		case err := <-d.watcher.Errors:
			d.logger.Errorf("Watcher error: %v", err)

		case <-ticker.C:
			// Periodic enforcement
			d.enforce()
		}
	}
}

// Stop stops the daemon
func (d *Daemon) Stop() {
	d.logger.Info("Stopping configlock daemon")
	close(d.stopCh)
	if d.watcher != nil {
		d.watcher.Close()
	}
	d.logger.Close()
}

// setupWatchers sets up file system watchers for all locked paths
func (d *Daemon) setupWatchers() error {
	// Remove all existing watches
	for _, path := range d.watcher.WatchList() {
		d.watcher.Remove(path)
	}

	// Add watches for all locked paths and their parent directories
	for _, path := range d.cfg.LockedPaths {
		// Watch the path itself
		if err := d.addWatch(path); err != nil {
			d.logger.Warnf("Failed to watch %s: %v", path, err)
		}

		// Watch parent directory to detect if file is deleted/recreated
		parent := filepath.Dir(path)
		if err := d.addWatch(parent); err != nil {
			d.logger.Warnf("Failed to watch parent %s: %v", parent, err)
		}
	}

	return nil
}

// addWatch adds a path to the watcher
func (d *Daemon) addWatch(path string) error {
	// Check if path exists
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	// If it's a directory, watch it
	if info.IsDir() {
		return d.watcher.Add(path)
	}

	// For files, watch the file itself
	return d.watcher.Add(path)
}

// enforce applies locks to all configured paths if within work hours
func (d *Daemon) enforce() {
	// Reload config to get latest changes
	cfg, err := config.Load()
	if err != nil {
		d.logger.Errorf("Failed to reload config during enforcement: %v", err)
		return
	}
	d.cfg = cfg

	// Clean expired temporary exclusions
	d.cfg.CleanExpiredExcludes()
	if err := d.cfg.Save(); err != nil {
		d.logger.Errorf("Failed to save config after cleaning exclusions: %v", err)
	}

	// Check if within work hours
	if !d.cfg.IsWithinWorkHours() {
		d.logger.Info("Outside work hours, skipping enforcement")
		return
	}

	d.logger.Info("Enforcing locks (within work hours)")

	// Apply locks to all paths
	for _, path := range d.cfg.LockedPaths {
		if d.cfg.IsTemporarilyExcluded(path) {
			d.logger.Infof("Skipping temporarily excluded path: %s", path)
			continue
		}

		d.lockPath(path)
	}
}

// lockPath applies a lock to a specific path
func (d *Daemon) lockPath(path string) {
	// Check if path exists
	if _, err := os.Stat(path); err != nil {
		d.logger.Warnf("Path no longer exists: %s", path)
		return
	}

	// Skip if temporarily excluded
	if d.cfg.IsTemporarilyExcluded(path) {
		return
	}

	// Apply lock
	if err := locker.Lock(path); err != nil {
		d.logger.Errorf("Failed to lock %s: %v", path, err)
	} else {
		d.logger.Infof("Locked: %s", path)
	}
}

// CollectFilesRecursively collects all files in a directory, skipping .git and .jj
func CollectFilesRecursively(root string) ([]string, error) {
	var files []string

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip .git and .jj directories
		if d.IsDir() {
			name := d.Name()
			if name == ".git" || name == ".jj" {
				return filepath.SkipDir
			}
		}

		// Add files (not directories)
		if !d.IsDir() {
			// Also skip if path contains /.git/ or /.jj/
			if !strings.Contains(path, "/.git/") && !strings.Contains(path, "/.jj/") {
				absPath, err := filepath.Abs(path)
				if err != nil {
					return err
				}
				files = append(files, absPath)
			}
		}

		return nil
	})

	return files, err
}
