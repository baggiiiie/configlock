# ConfigLock

A CLI tool and daemon that prevents editing of specified config files or directories during work hours using system-level immutable flags. Perfect for preventing impulsive config tinkering when you should be working!

## Features

- 🔒 **Immutable File Locking**: Uses system-level immutable flags (`chattr +i` on Linux, `chflags schg` on macOS)
- ⏰ **Work Hours Enforcement**: Automatically locks files during configurable work hours (default 8 AM - 5 PM, weekdays only)
- 🔄 **Real-time Monitoring**: File system watcher detects and re-locks files immediately if modified
- ⌨️ **Typing Challenge**: Anti-impulsive typing challenge for unlock operations (no copy-paste allowed!)
- 🔧 **Flexible Management**: Temporary unlocks with configurable durations
- 🚀 **Cross-Platform**: Supports Linux and macOS
- 🤖 **System Service**: Runs as a background daemon (systemd on Linux, launchd on macOS)

## Installation

### Quick Install (Recommended)

Install ConfigLock with a single command:

```bash
curl -sSL https://raw.githubusercontent.com/baggiiiie/configlock/main/configlock/install.sh | bash
```

This will automatically:
- Detect your OS and architecture
- Download the latest release
- Install to `/usr/local/bin/`

After installation, run:
```bash
configlock init
```

### Alternative: Download Pre-built Binary

Download the latest release for your platform from the [releases page](https://github.com/baggiiiie/configlock/releases):

```bash
# Download for your platform
# Linux AMD64
curl -LO https://github.com/baggiiiie/configlock/releases/latest/download/configlock-linux-amd64

# macOS ARM64 (Apple Silicon)
curl -LO https://github.com/baggiiiie/configlock/releases/latest/download/configlock-darwin-arm64

# Make executable and move to PATH
chmod +x configlock-*
sudo mv configlock-* /usr/local/bin/configlock
```

### Build from Source

Prerequisites:
- Go 1.24 or later
- Linux or macOS

```bash
# Clone or navigate to the configlock directory
cd configlock

# Build the binary
go build -o configlock

# Install to /usr/local/bin
sudo mv configlock /usr/local/bin/

# Initialize and install the daemon
configlock init
```

## Usage

### Initial Setup

Run the initialization command to set up ConfigLock:

```bash
configlock init
```

This will:
1. Create the config directory at `~/.config/configlock/`
2. Prompt you for work hours - you can enter either:
   - Simple time range: Just enter start time (e.g., `0800` or `08:00`), then end time
   - Cron schedule: Use `cron:` prefix (e.g., `cron:0 8-17 * * 1-5`)
3. Prompt you for temp unlock duration (default 5 minutes)
4. Install and start the daemon as a system service
5. Automatically add the config file itself to the lock list

**Time Input Formats**:
- Simple times: `HH:MM` or `HHMM` (e.g., `14:30` or `1430`) - the colon is optional!
- Cron schedule: `cron:MIN HOUR DAY MONTH WEEKDAY` (e.g., `cron:0 8-17 * * 1-5`)
- Invalid input will prompt you to retry

### Adding Files/Directories to Lock

Add individual files:

```bash
configlock add ~/.zshrc
configlock add ~/.vimrc
```

Add entire directories (recursively, excluding `.git/` and `.jj/`):

```bash
configlock add ~/.config/nvim
configlock add ~/dotfiles
```

**Note**: When adding directories, all files are collected recursively. Backup files (`.bak`) are created by default. Use `--no-backup` to skip backups.

### Viewing Locked Paths

List all locked paths:

```bash
configlock list
```

View current status:

```bash
configlock status
```

### Editing Work Hours

Need to change your work hours or switch between simple time range and cron schedule mode?

```bash
configlock edit time
```

This will:
1. Show your current work hours configuration
2. Prompt you to update work hours - you can enter either:
   - Simple time range: Enter start time (e.g., `0800`), then end time
   - Cron schedule: Use `cron:` prefix (e.g., `cron:0 9-17 * * 1-5`)
   - Press Enter to keep current configuration
3. Optionally update the temporary unlock duration
4. Invalid input will prompt you to retry

After updating, restart the daemon for changes to take effect:

```bash
configlock stop
configlock start
```

### Temporarily Unlocking Files

Need to edit something urgently? Temporarily unlock a path:

```bash
configlock temp-unlock ~/.zshrc
```

With custom duration:

```bash
configlock temp-unlock ~/.zshrc --duration 10
```

**Important**: This requires completing a typing challenge to prevent impulsive actions!

### Removing Files from Lock List

Remove a path from the lock list permanently:

```bash
configlock rm ~/.config/nvim
```

**Important**: This also requires completing the typing challenge!

## Configuration

The configuration file is stored at `~/.config/configlock/config.json`.

### Simple Time Range Mode

```json
{
  "locked_paths": [
    "/home/user/.zshrc",
    "/home/user/.config/nvim/init.lua"
  ],
  "start_time": "08:00",
  "end_time": "17:00",
  "cron_schedule": "",
  "temp_duration": 5,
  "temp_excludes": {}
}
```

### Cron Schedule Mode

For more complex schedules, you can use cron expressions:

```json
{
  "locked_paths": [
    "/home/user/.zshrc",
    "/home/user/.config/nvim/init.lua"
  ],
  "start_time": "",
  "end_time": "",
  "cron_schedule": "0 8-17 * * 1-5",
  "temp_duration": 5,
  "temp_excludes": {}
}
```

### Configuration Fields

- `locked_paths`: Array of absolute paths to lock
- `start_time`: Work start time in HH:MM format (used when `cron_schedule` is empty)
- `end_time`: Work end time in HH:MM format (used when `cron_schedule` is empty)
- `cron_schedule`: Optional cron expression for advanced scheduling (takes priority over `start_time`/`end_time`)
- `temp_duration`: Default duration for temporary unlocks (minutes)
- `temp_excludes`: Map of temporarily excluded paths with expiration timestamps

### Cron Schedule Examples

Cron format: `MIN HOUR DAY MONTH WEEKDAY`

When using cron schedules during `configlock init` or `configlock edit time`, prefix with `cron:`:

- `cron:0 8-17 * * 1-5` - Every hour from 8am-5pm on weekdays
- `cron:0 9-12,14-17 * * 1-5` - 9am-12pm and 2pm-5pm on weekdays (excluding lunch hour)
- `cron:0 10-18 * * 1-6` - 10am-6pm Monday through Saturday
- `cron:*/30 8-17 * * 1-5` - Every 30 minutes from 8am-5pm on weekdays

In the config file, the cron expression is stored without the `cron:` prefix in the `cron_schedule` field.

**Note**: The config file itself is automatically locked to prevent cheating!

## How It Works

### Locking Mechanism

ConfigLock uses system-level immutable flags to lock files:

- **Linux**: `chattr +i -R` (requires ext4/ext3/xfs filesystem)
- **macOS**: `chflags schg -R`

If immutable flags are unavailable, it falls back to `chmod 0444` (read-only).

### Daemon Behavior

The daemon runs in the background and:

1. Monitors all locked paths using `fsnotify` for instant reactions
2. Re-applies locks immediately if files are modified during work hours
3. Performs periodic enforcement sweeps every 30 seconds
4. Cleans up expired temporary exclusions
5. Only enforces locks during work hours on weekdays

### Typing Challenge

To prevent impulsive actions, `rm` and `temp-unlock` commands require typing this statement line-by-line:

```
I UNDERSTAND THIS ACTION WILL DECREASE,
AND POTENTIALLY ELIMINATE, MY PRODUCTIVITY.
I UNDERSTAND THE RISK INVOLVED,
AND I AM WILLING TO PROCEED.
```

- Each line must be typed exactly (case-sensitive)
- Characters appear with a typewriter effect to prevent copy-paste
- Maximum 3 attempts per line
- Challenge fails if too many incorrect attempts

## Service Management

The daemon is installed as a user service:

- **Linux**: `~/.config/systemd/user/configlock.service`
- **macOS**: `~/Library/LaunchAgents/com.configlock.daemon.plist`

### Manual Service Control (if needed)

**Linux (systemd)**:
```bash
systemctl --user status configlock
systemctl --user restart configlock
systemctl --user stop configlock
```

**macOS (launchd)**:
```bash
launchctl list | grep configlock
launchctl kickstart -k gui/$(id -u)/com.configlock.daemon
launchctl stop gui/$(id -u)/com.configlock.daemon
```

## Logging

Logs are written to:
- **Linux**: `~/.local/share/configlock/configlock.log`
- **macOS**: `~/Library/Logs/configlock.log`

Log entries include:
- Lock enforcement actions
- File system events detected
- Errors and warnings
- Service start/stop events

Logs automatically rotate when they exceed 10MB.

## Troubleshooting

### Locks not applying

1. Check daemon status: `systemctl --user status configlock` (Linux) or `launchctl list | grep configlock` (macOS)
2. Check logs for errors
3. Verify you're within work hours: `configlock status`
4. Ensure files exist at the specified paths

### Can't unlock files

If you need to force unlock files:

```bash
# Linux
chattr -i -R /path/to/file

# macOS
chflags -R noschg /path/to/file
```

Then remove from config or use `temp-unlock`.

### Permission errors

Note: Some operations may require elevated permissions:
- Setting immutable flags (chattr/chflags) on system files
- Installing system services
- If you encounter permission errors, you can either run with `sudo` or ensure files are owned by your user

## Uninstalling

To remove ConfigLock:

```bash
# Stop and uninstall the daemon
# Linux
systemctl --user stop configlock
systemctl --user disable configlock
rm ~/.config/systemd/user/configlock.service
systemctl --user daemon-reload

# macOS
launchctl unload ~/Library/LaunchAgents/com.configlock.daemon.plist
rm ~/Library/LaunchAgents/com.configlock.daemon.plist

# Remove config and binary
rm -rf ~/.config/configlock
rm /usr/local/bin/configlock
```

## Development

### Project Structure

```
configlock/
├── main.go                 # Entry point
├── cmd/                    # Cobra commands
│   ├── root.go
│   ├── init.go
│   ├── add.go
│   ├── rm.go
│   ├── tempunlock.go
│   ├── status.go
│   ├── list.go
│   └── daemon_cmd.go
├── internal/
│   ├── config/            # Config management
│   ├── locker/            # File locking logic
│   ├── challenge/         # Typing challenge
│   ├── daemon/            # Daemon implementation
│   ├── logger/            # Logging utilities
│   └── service/           # Service management
└── docs/                  # Specification documents
```

### Building

Build for current platform:
```bash
# Simple build
go build -o configlock

# Build with version info
go build -ldflags "-X github.com/baggiiiie/configlock/cmd.version=1.0.0" -o configlock

# Optimized build (smaller binary)
CGO_ENABLED=0 go build -ldflags "-s -w -X github.com/baggiiiie/configlock/cmd.version=1.0.0" -trimpath -o configlock
```

Cross-platform builds:
```bash
# Linux AMD64
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -trimpath -o configlock-linux-amd64

# macOS ARM64 (Apple Silicon)
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "-s -w" -trimpath -o configlock-darwin-arm64
```

### Releasing

Releases are automated via GitHub Actions. To create a new release:

1. **Create a GitHub release**:
   ```bash
   # Using GitHub CLI (recommended)
   gh release create configlock-v1.0.0 --generate-notes --title "ConfigLock v1.0.0"

   # Or create manually via GitHub web UI
   ```

2. **GitHub Actions will automatically**:
   - Build binaries for all platforms (linux/darwin × amd64/arm64)
   - Upload all binaries to the release

3. **Users can then install** with:
   ```bash
   curl -sSL https://raw.githubusercontent.com/baggiiiie/configlock/main/configlock/install.sh | bash
   ```

### Dependencies

- `github.com/spf13/cobra` - CLI framework
- `github.com/fsnotify/fsnotify` - File system notifications
- `github.com/kardianos/service` - Cross-platform service management
- `github.com/robfig/cron/v3` - Cron expression parsing

## License

See the repository license file.

## Contributing

Contributions are welcome! Please ensure:
- Code follows the existing structure
- Changes align with the design documents in `docs/`
- Cross-platform compatibility is maintained

## Acknowledgments

Built as a productivity tool for developers who can't resist tinkering with their configs during work hours. You know who you are. 😉
