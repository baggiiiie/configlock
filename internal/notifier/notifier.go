package notifier

import (
	"github.com/gen2brain/beeep"
)

type Notifier struct {
	appName string
}

// New creates a new notifier instance
func New(appName string) *Notifier {
	return &Notifier{appName: appName}
}

// Notify sends a system notification using the beeep library
// which provides cross-platform support for macOS, Linux, Windows, and FreeBSD
func (n *Notifier) Notify(title, message string) error {
	// beeep.Notify sends a system notification with title, message, and optional icon
	// The empty string means no custom icon will be used
	return beeep.Notify(title, message, "")
}
