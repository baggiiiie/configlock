package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/baggiiiie/configlock/internal/logger"
	"github.com/spf13/cobra"
)

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "Tail the configlock log file",
	Long:  `Tail the configlock log file to see immutability changes and daemon activity in real-time.`,
	RunE:  runLogs,
}

func init() {
	rootCmd.AddCommand(logsCmd)
}

func runLogs(cmd *cobra.Command, args []string) error {
	// Get logger instance and log path
	log := logger.GetLogger()
	logPath := log.GetLogPath()

	if logPath == "" {
		return fmt.Errorf("log file path not available")
	}

	// Check if log file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		return fmt.Errorf("log file does not exist: %s", logPath)
	}

	fmt.Printf("Tailing log file: %s\n", logPath)
	fmt.Println("Press Ctrl+C to stop")
	fmt.Println("---")

	// Open the log file
	file, err := os.Open(logPath)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	// Seek to the end of the file
	if _, err := file.Seek(0, io.SeekEnd); err != nil {
		return fmt.Errorf("failed to seek to end of file: %w", err)
	}

	// Create a reader
	reader := bufio.NewReader(file)

	// Set up signal handling for Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Create a ticker for polling the file
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-sigChan:
			fmt.Println("\nStopping log tail...")
			return nil
		case <-ticker.C:
			// Read new lines
			for {
				line, err := reader.ReadString('\n')
				if err != nil {
					// No more lines to read, break and wait for next tick
					break
				}
				fmt.Print(line)
			}
		}
	}
}
