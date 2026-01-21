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
	"github.com/baggiiiie/configlock/internal/notifier"
	"github.com/fsnotify/fsnotify"
)

type Daemon struct {
	cfg       *config.Config
	watcher   *fsnotify.Watcher
	logger    *logger.Logger
	notifier  *notifier.Notifier
	stopCh    chan struct{}
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
		cfg:       cfg,
		watcher:   watcher,
		logger:    logger.GetLogger(),
		notifier:  notifier.New("ConfigLock"),
		stopCh:    make(chan struct{}),
	}, nil
}

// Start starts the daemon
//
// The daemon uses two mechanisms to ensure immutable flags stay applied:
//  1. File system watching (fsnotify) - provides instant reaction to changes
//  2. Periodic sweep (every 30 seconds) - catches changes that fsnotify might miss,
//     such as manual flag removal via 'sudo chattr -i' or 'sudo chflags noschg'
func (d *Daemon) Start() error {
	d.logger.Info("Starting configlock daemon")

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

	// Set up file watchers for instant reaction to file system changes
	if err := d.setupWatchers(); err != nil {
		d.logger.Errorf("Failed to setup watchers: %v", err)
		// Continue anyway, periodic enforcement will still work
	}

	// Start periodic enforcement (runs every 30 seconds)
	// This catches attribute changes that fsnotify might not detect
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
				// SIGTERM or SIGINT - graceful shutdown with unlock
				d.gracefulShutdown()
				return nil
			}

		case event := <-d.watcher.Events:
			// Ignore events on configlock's own config file to prevent log spam
			configDir := config.GetConfigDir()
			configPath := config.GetConfigPath()
			if event.Name != configPath && event.Name != configDir &&
				!strings.HasPrefix(event.Name, configDir+string(filepath.Separator)) {
				d.logger.Infof("File event detected: %s %s", event.Op, event.Name)
			}

			// Re-apply locks immediately on any file change detected by fsnotify
			// This provides instant reaction to modifications, deletions, renames, etc.
			if d.cfg.IsWithinWorkHours() {
				// Find which locked path this event relates to and re-lock it
				d.handleFileEvent(event.Name)
			}

		case err := <-d.watcher.Errors:
			d.logger.Errorf("Watcher error: %v", err)

		case <-ticker.C:
			// Periodic enforcement
			d.enforce()
		}
	}
}

// gracefulShutdown unlocks all configured paths and stops the daemon
func (d *Daemon) gracefulShutdown() {
	d.logger.Info("Graceful shutdown initiated - unlocking all paths")

	// Unlock all configured paths
	for _, path := range d.cfg.LockedPaths {
		d.logger.Infof("Unlocking path: %s", path)
		if err := locker.Unlock(path); err != nil {
			d.logger.Errorf("Failed to unlock %s: %v", path, err)
		}
	}

	d.logger.Info("All paths unlocked, stopping daemon")
	d.Stop()
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

// enforce applies locks to all configured paths if within lock hours
func (d *Daemon) enforce() {
	// Reload config to get latest changes
	cfg, err := config.Load()
	if err != nil {
		d.logger.Errorf("Failed to reload config during enforcement: %v", err)
		return
	}
	d.cfg = cfg

	// Clean expired temporary exclusions and save only if something was cleaned
	if d.cfg.CleanExpiredExcludes() {
		if err := d.cfg.Save(); err != nil {
			d.logger.Errorf("Failed to save config after cleaning exclusions: %v", err)
		}
	}

	// Check if within lock hours
	if !d.cfg.IsWithinWorkHours() {
		d.logger.Info("Outside lock hours, skipping enforcement")
		return
	}

	d.logger.Info("Enforcing locks (within lock hours)")

	// Apply locks to all paths and verify they're locked
	for _, path := range d.cfg.LockedPaths {
		if d.cfg.IsTemporarilyExcluded(path) {
			d.logger.Infof("Skipping temporarily excluded path: %s", path)
			continue
		}

		// Verify if lock is still in place
		if locked, err := locker.IsLocked(path); err == nil && !locked {
			d.logger.Warnf("Lock removed from %s, re-applying", path)
		}

		d.lockPath(path)
	}
}

// handleFileEvent processes a file system event and re-locks the appropriate path
func (d *Daemon) handleFileEvent(eventPath string) {
	// Ignore events on configlock's own config file to prevent feedback loop
	configDir := config.GetConfigDir()
	configPath := config.GetConfigPath()
	if eventPath == configPath || eventPath == configDir ||
		strings.HasPrefix(eventPath, configDir+string(filepath.Separator)) {
		return
	}

	// Find all locked paths that match or contain this event path
	for _, lockedPath := range d.cfg.LockedPaths {
		// Skip if temporarily excluded
		if d.cfg.IsTemporarilyExcluded(lockedPath) {
			continue
		}

		// Check if event path is the locked path itself or within it
		if eventPath == lockedPath || strings.HasPrefix(eventPath, lockedPath+string(filepath.Separator)) {
			d.logger.Infof("Event detected on locked path %s, re-applying lock", lockedPath)
			d.sendManualChangeNotification(lockedPath)
			d.lockPath(lockedPath)
		} else if filepath.Dir(eventPath) == filepath.Dir(lockedPath) {
			// Event in parent directory (e.g., file was deleted/recreated)
			d.logger.Infof("Event detected in parent of locked path %s, re-applying lock", lockedPath)
			d.sendManualChangeNotification(lockedPath)
			d.lockPath(lockedPath)
		}
	}
}

// sendManualChangeNotification sends a system notification when manual changes are detected
func (d *Daemon) sendManualChangeNotification(path string) {
	title := "ConfigLock Alert"
	message := fmt.Sprintf("Detected manual change to locked file: %s\nConfigLock will re-apply the lock.", filepath.Base(path))
	
	if err := d.notifier.Notify(title, message); err != nil {
		d.logger.Warnf("Failed to send notification: %v", err)
		// Don't fail the entire operation if notification fails
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
