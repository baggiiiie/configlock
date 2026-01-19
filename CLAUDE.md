# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Architecture

ConfigLock is a Go CLI tool and daemon that locks config files during work hours using system-level immutable flags.

### Code Structure

- `main.go` - Entry point, executes root Cobra command
- `cmd/` - Cobra CLI commands (init, add, rm, temp-unlock, status, list, start, stop, daemon, etc.)
- `internal/config/` - Config file management (`~/.config/configlock/config.json`)
- `internal/locker/` - File locking logic (chattr on Linux, chflags on macOS, chmod fallback)
- `internal/daemon/` - Background daemon with fsnotify file watcher and periodic enforcement
- `internal/challenge/` - Typing challenge implementation for rm/temp-unlock commands
- `internal/service/` - System service management (systemd on Linux, launchd on macOS)
- `internal/logger/` - Structured logging with rotation
- `internal/fileutil/` - File utilities (recursive directory walking, backup creation)

### Key Dependencies

- `github.com/spf13/cobra` - CLI framework
- `github.com/fsnotify/fsnotify` - File system event monitoring
- `github.com/kardianos/service` - Cross-platform service management
- `github.com/robfig/cron/v3` - Cron expression parsing for work hours

### Platform-Specific Behavior

- **Linux**: Uses `chattr +i -R` for immutable flags, systemd user service at `~/.config/systemd/user/configlock.service`, logs to `~/.local/share/configlock/configlock.log`
- **macOS**: Uses `chflags schg -R` for immutable flags, launchd agent at `~/Library/LaunchAgents/com.configlock.daemon.plist`, logs to `~/Library/Logs/configlock.log`
- Falls back to `chmod 0444` if immutable flags fail

### Daemon Architecture

The daemon (`configlock daemon`, hidden command) runs two concurrent mechanisms:
1. **File Event Handler**: fsnotify watcher on locked paths, immediately re-locks on any file event during work hours
2. **Periodic Sweep**: Every 30 seconds, enforces locks on all paths and cleans expired temp excludes

### Config File Structure

Located at `~/.config/configlock/config.json`:
- `locked_paths`: Array of absolute paths to lock
- `start_time`/`end_time`: Simple time range (HH:MM format)
- `cron_schedule`: Alternative cron expression for complex schedules
- `temp_duration`: Default minutes for temporary unlocks
- `temp_excludes`: Map of path to expiration timestamp
