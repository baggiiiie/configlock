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
	cfg      *config.Config
	watcher  *fsnotify.Watcher
	logger   *logger.Logger
	notifier *notifier.Notifier
	stopCh   chan struct{}
	active   bool // true when within work hours and watchers are set up
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
		cfg:      cfg,
		watcher:  watcher,
		logger:   logger.GetLogger(),
		notifier: notifier.New("ConfigLock"),
		stopCh:   make(chan struct{}),
	}, nil
}

// Start starts the daemon
//
// The daemon uses two mechanisms to ensure immutable flags stay applied:
//  1. File system watching (fsnotify) - provides instant reaction to changes
//  2. Periodic sweep (every 30 seconds) - catches changes that fsnotify might miss,
//     such as manual flag removal via 'sudo chattr -i' or 'sudo chflags noschg'
//
// Outside work hours, the daemon sleeps until work hours start.
func (d *Daemon) Start() error {
	d.logger.Info("Starting configlock daemon")

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT, syscall.SIGHUP)

	// Create timer for next check
	timer := time.NewTimer(0) // fires immediately for initial check
	defer timer.Stop()

	for {
		select {
		case <-d.stopCh:
			d.logger.Info("Daemon stopped")
			return nil

		case sig := <-sigCh:
			d.logger.Infof("Received signal: %v", sig)
			if sig == syscall.SIGHUP {
				d.logger.Info("Reloading configuration")
				cfg, err := config.Load()
				if err != nil {
					d.logger.Errorf("Failed to reload config: %v", err)
				} else {
					d.cfg = cfg
					if d.active {
						d.setupWatchers()
					}
				}
			} else {
				d.gracefulShutdown()
				return nil
			}

		case event := <-d.watcher.Events:
			if !d.active {
				continue
			}
			// Ignore events on configlock's own config file
			configDir := config.GetConfigDir()
			configPath := config.GetConfigPath()
			if event.Name != configPath && event.Name != configDir &&
				!strings.HasPrefix(event.Name, configDir+string(filepath.Separator)) {
				d.logger.Infof("File event detected: %s %s", event.Op, event.Name)
			}
			d.handleFileEvent(event.Name)

		case err := <-d.watcher.Errors:
			d.logger.Errorf("Watcher error: %v", err)

		case <-timer.C:
			withinWorkHours := d.cfg.IsWithinWorkHours()

			if withinWorkHours && !d.active {
				// Transition: entering work hours
				d.activate()
				timer.Reset(30 * time.Second)
			} else if !withinWorkHours && d.active {
				// Transition: leaving work hours
				d.deactivate()
				sleepDuration := d.cfg.TimeUntilWorkHours()
				d.logger.Infof("Sleeping until work hours start (%s)", sleepDuration.Round(time.Minute))
				timer.Reset(sleepDuration)
			} else if d.active {
				// Already active, enforce and check again in 30s
				d.enforce()
				timer.Reset(30 * time.Second)
			} else {
				// Still inactive, sleep until work hours
				sleepDuration := d.cfg.TimeUntilWorkHours()
				d.logger.Infof("Outside work hours, sleeping until start (%s)", sleepDuration.Round(time.Minute))
				timer.Reset(sleepDuration)
			}
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

// activate sets up watchers and enforces locks when entering work hours
func (d *Daemon) activate() {
	d.logger.Info("Entering work hours, activating")
	d.active = true
	if err := d.setupWatchers(); err != nil {
		d.logger.Errorf("Failed to setup watchers: %v", err)
	}
	d.enforce()
}

// deactivate removes watchers and unlocks paths when leaving work hours
func (d *Daemon) deactivate() {
	d.logger.Info("Leaving work hours, deactivating")
	d.active = false
	d.clearWatchers()
	// Unlock all paths
	for _, path := range d.cfg.LockedPaths {
		if err := locker.Unlock(path); err != nil {
			d.logger.Errorf("Failed to unlock %s: %v", path, err)
		}
	}
}

// clearWatchers removes all file system watchers
func (d *Daemon) clearWatchers() {
	for _, path := range d.watcher.WatchList() {
		d.watcher.Remove(path)
	}
}

// setupWatchers sets up file system watchers for all locked paths
func (d *Daemon) setupWatchers() error {
	// Remove all existing watches
	for _, path := range d.watcher.WatchList() {
		d.watcher.Remove(path)
	}

	// Add watches for all locked paths
	for _, path := range d.cfg.LockedPaths {
		if err := d.addWatch(path); err != nil {
			d.logger.Warnf("Failed to watch %s: %v", path, err)
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

	d.logger.Info("Enforcing locks")

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
