package cmd

import (
	"fmt"

	"github.com/baggiiiie/configlock/internal/daemon"
	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:    "daemon",
	Short:  "Run the daemon (internal use only)",
	Long:   `This command is used internally by the system service. Do not run it manually.`,
	Hidden: true,
	RunE:   runDaemon,
}

func init() {
	rootCmd.AddCommand(daemonCmd)
}

func runDaemon(cmd *cobra.Command, args []string) error {
	// Create and start daemon
	d, err := daemon.New()
	if err != nil {
		return fmt.Errorf("failed to create daemon: %w", err)
	}

	return d.Start()
}
