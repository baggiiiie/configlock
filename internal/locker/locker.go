package locker

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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
	if _, err := os.Stat(realPath); err != nil {
		return fmt.Errorf("path does not exist: %s", realPath)
	}

	switch runtime.GOOS {
	case "linux":
		return lockLinux(realPath)
	case "darwin":
		return lockDarwin(realPath)
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
	if _, err := os.Stat(realPath); err != nil {
		return fmt.Errorf("path does not exist: %s", realPath)
	}

	switch runtime.GOOS {
	case "linux":
		return unlockLinux(realPath)
	case "darwin":
		return unlockDarwin(realPath)
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}
}

// lockLinux applies immutable flag on Linux
func lockLinux(path string) error {
	cmd := exec.Command("chattr", "+i", "-R", path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Try fallback to chmod
		if err := fallbackLock(path); err != nil {
			return fmt.Errorf("chattr failed and fallback failed: %v, output: %s", err, string(output))
		}
		return nil
	}
	return nil
}

// unlockLinux removes immutable flag on Linux
func unlockLinux(path string) error {
	cmd := exec.Command("chattr", "-i", "-R", path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Try fallback to chmod
		if err := fallbackUnlock(path); err != nil {
			return fmt.Errorf("chattr failed and fallback failed: %v, output: %s", err, string(output))
		}
		return nil
	}
	return nil
}

// lockDarwin applies immutable flag on macOS
func lockDarwin(path string) error {
	cmd := exec.Command("chflags", "-R", "schg", path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Try fallback to chmod
		if err := fallbackLock(path); err != nil {
			return fmt.Errorf("chflags failed and fallback failed: %v, output: %s", err, string(output))
		}
		return nil
	}
	return nil
}

// unlockDarwin removes immutable flag on macOS
func unlockDarwin(path string) error {
	cmd := exec.Command("chflags", "-R", "noschg", path)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Try fallback to chmod
		if err := fallbackUnlock(path); err != nil {
			return fmt.Errorf("chflags failed and fallback failed: %v, output: %s", err, string(output))
		}
		return nil
	}
	return nil
}

// fallbackLock sets read-only permissions as fallback
func fallbackLock(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	if info.IsDir() {
		// Recursively set read-only for all files in directory
		return filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			// Skip .git and .jj directories
			if info.IsDir() && (strings.Contains(p, "/.git/") || strings.Contains(p, "/.jj/")) {
				return filepath.SkipDir
			}
			if !info.IsDir() {
				return os.Chmod(p, 0444)
			}
			return nil
		})
	}

	return os.Chmod(path, 0444)
}

// fallbackUnlock sets writable permissions as fallback
func fallbackUnlock(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	if info.IsDir() {
		// Recursively set writable for all files in directory
		return filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			// Skip .git and .jj directories
			if info.IsDir() && (strings.Contains(p, "/.git/") || strings.Contains(p, "/.jj/")) {
				return filepath.SkipDir
			}
			if !info.IsDir() {
				return os.Chmod(p, 0644)
			}
			return nil
		})
	}

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
