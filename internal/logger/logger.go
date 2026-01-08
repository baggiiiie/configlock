package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

const maxLogSize = 10 * 1024 * 1024 // 10MB

type Logger struct {
	mu       sync.Mutex
	file     *os.File
	logger   *log.Logger
	logPath  string
	disabled bool
}

var (
	defaultLogger *Logger
	once          sync.Once
)

// GetLogger returns the default logger instance
func GetLogger() *Logger {
	once.Do(func() {
		defaultLogger = &Logger{}
		if err := defaultLogger.Init(); err != nil {
			// If we can't initialize the logger, just disable it
			defaultLogger.disabled = true
		}
	})
	return defaultLogger
}

// Init initializes the logger
func (l *Logger) Init() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Determine log path based on OS
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	var logPath string
	switch runtime.GOOS {
	case "linux":
		// Use XDG_DATA_HOME or default to ~/.local/share
		logDir := filepath.Join(home, ".local", "share", "configlock")
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return fmt.Errorf("failed to create log directory: %w", err)
		}
		logPath = filepath.Join(logDir, "configlock.log")
	case "darwin":
		logDir := filepath.Join(home, "Library", "Logs")
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return fmt.Errorf("failed to create log directory: %w", err)
		}
		logPath = filepath.Join(logDir, "configlock.log")
	default:
		return fmt.Errorf("unsupported OS: %s", runtime.GOOS)
	}

	l.logPath = logPath

	// Open or create log file
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	l.file = file
	l.logger = log.New(file, "", 0)

	return nil
}

// Close closes the log file
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// log writes a log entry with timestamp and level
func (l *Logger) log(level, message string) {
	if l.disabled {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// Check if log rotation is needed
	if l.file != nil {
		info, err := l.file.Stat()
		if err == nil && info.Size() > maxLogSize {
			l.rotate()
		}
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	entry := fmt.Sprintf("[%s] [%s] %s\n", timestamp, level, message)

	if l.logger != nil {
		l.logger.Print(entry)
	}
}

// rotate rotates the log file when it exceeds maxLogSize
func (l *Logger) rotate() {
	if l.file == nil {
		return
	}

	// Close current file
	l.file.Close()

	// Rename old log file
	backupPath := l.logPath + ".old"
	os.Remove(backupPath) // Remove old backup if exists
	os.Rename(l.logPath, backupPath)

	// Open new log file
	file, err := os.OpenFile(l.logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		l.disabled = true
		return
	}

	l.file = file
	l.logger = log.New(file, "", 0)
}

// Info logs an info message
func (l *Logger) Info(message string) {
	l.log("INFO", message)
}

// Warn logs a warning message
func (l *Logger) Warn(message string) {
	l.log("WARN", message)
}

// Error logs an error message
func (l *Logger) Error(message string) {
	l.log("ERROR", message)
}

// Infof logs a formatted info message
func (l *Logger) Infof(format string, args ...interface{}) {
	l.Info(fmt.Sprintf(format, args...))
}

// Warnf logs a formatted warning message
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.Warn(fmt.Sprintf(format, args...))
}

// Errorf logs a formatted error message
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.Error(fmt.Sprintf(format, args...))
}

// SetOutput sets an additional output writer for the logger
func (l *Logger) SetOutput(w io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.logger != nil && l.file != nil {
		l.logger = log.New(io.MultiWriter(l.file, w), "", 0)
	}
}
