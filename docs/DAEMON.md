# Daemon Implementation Details

## Overview

The daemon runs continuously as a system service, enforcing immutable flags on locked paths during work hours.

### Entry Point

- Subcommand: `configlock daemon` (hidden from help).

### Configuration Structure

```go
type Config struct {
    LockedPaths    []string                       `json:"locked_paths"`
    StartTime      string                         `json:"start_time"`      // "08:00"
    EndTime        string                         `json:"end_time"`        // "17:00"
    TempDuration   int                            `json:"temp_duration"`   // minutes
    TempExcludes   map[string]time.Time           `json:"temp_excludes"`   // path -> expiration
}
```

## Main Loop

1. Load config (with file mutex to avoid race with CLI).
1. Set up fsnotify watcher for all locked paths and their parent directories.
1. Run two concurrent mechanisms:

- File Event Handler (instant reaction):
  - On any MODIFY/CREATE/REMOVE/CHMOD event for watched paths → immediately re-apply lock if within work hours.

- Periodic Sweep (every 30 seconds via time.Ticker):
  - Check if current time is weekday and within configured work hours (using time.Now().Local()).
  - Clean expired entries from temp_excludes.
  - For every path in locked_paths not temporarily excluded:
    - Apply immutable flag recursively:
      - Linux: chattr +i -R <path>
      - macOS: chflags schg -R <path>
  - Log action.

1. Log all significant events (lock applied, attempt detected, errors) to ~/.local/share/configlock/configlock.log (or ~/Library/Logs/configlock.log on macOS).

## Locking Functions

- Detect OS via runtime.GOOS.
- Use os/exec.Command to run chattr or chflags.
- Handle directories recursively.
- Fallback: If immutable flags fail (e.g., wrong filesystem), fall back to chmod 0444 and warn.

## Service Management

- Use github.com/kardianos/service package.
- Support install, uninstall, start, stop.
- init command handles installation and starting.

## Shutdown

- Trap SIGTERM/SIGINT.
- Optionally unlock all files on clean shutdown (configurable).

## Edge Cases

- Missing paths: Remove from config and log.
- Permission errors: Log and continue.
- Config changes: Reload on SIGHUP or periodic check.
