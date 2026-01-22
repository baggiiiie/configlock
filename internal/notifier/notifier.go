package notifier

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

type Notifier struct {
	appName string
}

// New creates a new notifier instance
func New(appName string) *Notifier {
	return &Notifier{appName: appName}
}

// Notify sends a system notification
func (n *Notifier) Notify(title, message string) error {
	switch runtime.GOOS {
	case "darwin":
		return n.notifyMacOS(title, message)
	case "linux":
		return n.notifyLinux(title, message)
	default:
		return fmt.Errorf("notifications not supported on %s", runtime.GOOS)
	}
}

// notifyMacOS sends a notification on macOS using osascript
func (n *Notifier) notifyMacOS(title, message string) error {
	// Escape single quotes in the strings
	title = strings.ReplaceAll(title, "'", "'\\''")
	message = strings.ReplaceAll(message, "'", "'\\''")

	script := fmt.Sprintf(`
display notification "%s" with title "%s"
`, message, title)

	cmd := exec.Command("osascript", "-e", script)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to send macOS notification: %w", err)
	}

	return nil
}

// notifyLinux sends a notification on Linux using notify-send or dbus
func (n *Notifier) notifyLinux(title, message string) error {
	// Try notify-send first (most common on Linux desktops)
	cmd := exec.Command("notify-send", title, message, "-u", "critical", "-t", "5000")
	if err := cmd.Run(); err == nil {
		return nil
	}

	// Fallback: try using dbus directly for systems without notify-send
	// This is a common notification daemon protocol
	cmd = exec.Command("gdbus", "call",
		"--session",
		"--dest=org.freedesktop.Notifications",
		"--object-path=/org/freedesktop/Notifications",
		"--method=org.freedesktop.Notifications.Notify",
		"configlock",     // app_name
		"0",              // replaces_id (0 = new notification)
		"dialog-warning", // app_icon
		title,            // summary
		message,          // body
		"[]",             // actions
		"{}",             // hints
		"5000")           // expire_timeout (5 seconds)

	if err := cmd.Run(); err == nil {
		return nil
	}

	// If both methods fail, check if we're in a headless environment
	// and fail silently as this is not critical
	if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		return fmt.Errorf("no display server available for notifications")
	}

	return fmt.Errorf("failed to send Linux notification: notify-send not available")
}
