# CLAUDE.md

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

### Daemon Architecture

The daemon (`configlock daemon`, hidden command) is work-hours-aware and operates in two states:

**Active (within work hours)**:

- Sets up fsnotify watchers on locked paths only (not parent directories)
- File events trigger immediate re-lock
- Periodic sweep every 30 seconds enforces locks and cleans expired temp excludes
- Only logs when actually applying a lock (skips if already locked)

**Inactive (outside work hours)**:

- No watchers, no enforcement
- Sleeps until next work hours start (calculated via `TimeUntilWorkHours()`)
- On transition out of work hours: reloads config, unlocks all paths, removes watchers

**Config reloading**: Only on SIGHUP signal, not during periodic enforcement

### Config File Structure

Located at `~/.config/configlock/config.json`:

- `locked_paths`: Array of absolute paths to lock
- `start_time`/`end_time`: Simple time range (HH:MM format)
- `cron_schedule`: Alternative cron expression for complex schedules
- `temp_duration`: Default minutes for temporary unlocks
- `temp_excludes`: Map of path to expiration timestamp
