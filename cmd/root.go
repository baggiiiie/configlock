package cmd

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/baggiiiie/configlock/internal/upgrade"
	"github.com/spf13/cobra"
)

var version = ""

var rootCmd = &cobra.Command{
	Use:   "configlock",
	Short: "ConfigLock - A productivity enforcer for config files",
	Long: `ConfigLock is a CLI tool and daemon that prevents editing of specified
config files or directories during lock hours using system-level immutable flags.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		// Skip upgrade check for daemon (runs in background) and help/version
		if cmd.Name() == "daemon" || cmd.Name() == "help" {
			return
		}
		upgrade.CheckForUpgrade(GetVersion())
	},
}

// SetVersion sets the version from main package
func SetVersion(v string) {
	version = v
	rootCmd.Version = GetVersion()
}

// GetVersion returns the version string with fallback logic
func GetVersion() string {
	if version != "" {
		return version
	}

	// Try to get version from build info
	if info, ok := debug.ReadBuildInfo(); ok {
		if info.Main.Version != "" && info.Main.Version != "(devel)" {
			return info.Main.Version
		}
	}

	return "unknown"
}

// Execute runs the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.CompletionOptions.DisableDefaultCmd = true
}
