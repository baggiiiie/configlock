package locker

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/baggiiiie/configlock/internal/fileutil"
	"github.com/baggiiiie/configlock/internal/logger"
)

// Lock applies immutable flags to a path recursively
func Lock(path string) error {
	// Resolve symlinks
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		// If symlink resolution fails, try with the original path
		realPath = path
	}

	// Check if path exists
	info, err := os.Stat(realPath)
	if err != nil {
		return fmt.Errorf("path does not exist: %s", realPath)
	}

	// If it's a directory, collect files respecting .gitignore and lock each file
	if info.IsDir() {
		files, err := fileutil.CollectFilesRecursively(realPath)
		if err != nil {
			return fmt.Errorf("failed to collect files: %w", err)
		}

		var lastErr error
		for _, file := range files {
			if err := lockFile(file); err != nil {
				lastErr = err
			}
		}
		return lastErr
	}

	// For single files, lock directly
	return lockFile(realPath)
}

// lockFile locks a single file (not a directory)
func lockFile(path string) error {
	switch runtime.GOOS {
	case "linux":
		return lockLinux(path)
	case "darwin":
		return lockDarwin(path)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// Unlock removes immutable flags from a path recursively
func Unlock(path string) error {
	// Resolve symlinks
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		// If symlink resolution fails, try with the original path
		realPath = path
	}

	// Check if path exists
	info, err := os.Stat(realPath)
	if err != nil {
		return fmt.Errorf("path does not exist: %s", realPath)
	}

	// If it's a directory, collect files respecting .gitignore and unlock each file
	if info.IsDir() {
		files, err := fileutil.CollectFilesRecursively(realPath)
		if err != nil {
			return fmt.Errorf("failed to collect files: %w", err)
		}

		var lastErr error
		for _, file := range files {
			if err := unlockFile(file); err != nil {
				lastErr = err
			}
		}
		return lastErr
	}

	// For single files, unlock directly
	return unlockFile(realPath)
}

// unlockFile unlocks a single file (not a directory)
func unlockFile(path string) error {
	switch runtime.GOOS {
	case "linux":
		return unlockLinux(path)
	case "darwin":
		return unlockDarwin(path)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// lockLinux applies immutable flag on Linux (for a single file)
func lockLinux(path string) error {
	cmd := exec.Command("chattr", "+i", path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Try fallback to chmod
		if err := fallbackLock(path); err != nil {
			return fmt.Errorf("chattr failed and fallback failed: %v, output: %s", err, string(output))
		}
		logger.GetLogger().Infof("LOCK (fallback): chattr +i %s", path)
		return nil
	}
	logger.GetLogger().Infof("LOCK: chattr +i %s", path)
	return nil
}

// unlockLinux removes immutable flag on Linux (for a single file)
func unlockLinux(path string) error {
	cmd := exec.Command("chattr", "-i", path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Try fallback to chmod
		if err := fallbackUnlock(path); err != nil {
			return fmt.Errorf("chattr failed and fallback failed: %v, output: %s", err, string(output))
		}
		logger.GetLogger().Infof("UNLOCK (fallback): chattr -i %s", path)
		return nil
	}
	logger.GetLogger().Infof("UNLOCK: chattr -i %s", path)
	return nil
}

// lockDarwin applies immutable flag on macOS (for a single file)
func lockDarwin(path string) error {
	cmd := exec.Command("chflags", "schg", path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Try fallback to chmod
		if err := fallbackLock(path); err != nil {
			return fmt.Errorf("chflags failed and fallback failed: %v, output: %s", err, string(output))
		}
		logger.GetLogger().Infof("LOCK (fallback): chflags schg %s", path)
		return nil
	}
	logger.GetLogger().Infof("LOCK: chflags schg %s", path)
	return nil
}

// unlockDarwin removes immutable flag on macOS (for a single file)
func unlockDarwin(path string) error {
	cmd := exec.Command("chflags", "noschg", path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Try fallback to chmod
		if err := fallbackUnlock(path); err != nil {
			return fmt.Errorf("chflags failed and fallback failed: %v, output: %s", err, string(output))
		}
		logger.GetLogger().Infof("UNLOCK (fallback): chflags noschg %s", path)
		return nil
	}
	logger.GetLogger().Infof("UNLOCK: chflags noschg %s", path)
	return nil
}

// fallbackLock sets read-only permissions as fallback (for a single file)
func fallbackLock(path string) error {
	return os.Chmod(path, 0444)
}

// fallbackUnlock sets writable permissions as fallback (for a single file)
func fallbackUnlock(path string) error {
	return os.Chmod(path, 0644)
}

// IsLocked checks if a path has immutable flags set
func IsLocked(path string) (bool, error) {
	// Resolve symlinks
	realPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		realPath = path
	}

	// Check if path exists
	if _, err := os.Stat(realPath); err != nil {
		return false, fmt.Errorf("path does not exist: %s", realPath)
	}

	switch runtime.GOOS {
	case "linux":
		return isLockedLinux(realPath)
	case "darwin":
		return isLockedDarwin(realPath)
	default:
		return false, fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// isLockedLinux checks if immutable flag is set on Linux
func isLockedLinux(path string) (bool, error) {
	// Use lsattr to check if immutable flag is set
	cmd := exec.Command("lsattr", path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// If lsattr is not available or fails, check permissions
		info, statErr := os.Stat(path)
		if statErr != nil {
			return false, statErr
		}
		// Check if read-only (fallback check)
		return info.Mode().Perm() == 0444, nil
	}

	// lsattr output format: "----i--------e----- /path/to/file"
	// Check if 'i' flag is present (5th character)
	outputStr := string(output)
	if len(outputStr) >= 5 {
		return strings.Contains(outputStr[:20], "i"), nil
	}

	return false, nil
}

// isLockedDarwin checks if immutable flag is set on macOS
func isLockedDarwin(path string) (bool, error) {
	// Use stat command to check file flags
	cmd := exec.Command("stat", "-f", "%Sf", path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// If stat fails, fallback to permission check
		info, statErr := os.Stat(path)
		if statErr != nil {
			return false, statErr
		}
		// Fallback to permission check
		return info.Mode().Perm() == 0444, nil
	}

	// Check if output contains "schg" flag
	outputStr := string(output)
	return strings.Contains(outputStr, "schg"), nil
}
