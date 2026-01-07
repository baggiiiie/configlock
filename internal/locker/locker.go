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
