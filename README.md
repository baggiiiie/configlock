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

### Prerequisites

- Go 1.21 or later
- Linux or macOS

### Build from Source

```bash
# Clone or navigate to the configlock directory
cd configlock

# Build the binary
go build -o configlock

# Move to a location in your PATH (optional)
mv configlock /usr/local/bin/

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
2. Prompt you for work hours (default 08:00 - 17:00)
3. Prompt you for temp unlock duration (default 5 minutes)
4. Install and start the daemon as a system service
5. Automatically add the config file itself to the lock list

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

The configuration file is stored at `~/.config/configlock/config.json`:

```json
{
  "locked_paths": [
    "/home/user/.zshrc",
    "/home/user/.config/nvim/init.lua"
  ],
  "start_time": "08:00",
  "end_time": "17:00",
  "temp_duration": 5,
  "temp_excludes": {}
}
```

### Configuration Fields

- `locked_paths`: Array of absolute paths to lock
- `start_time`: Work start time in HH:MM format
- `end_time`: Work end time in HH:MM format
- `temp_duration`: Default duration for temporary unlocks (minutes)
- `temp_excludes`: Map of temporarily excluded paths with expiration timestamps

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

```bash
# Build with version info
go build -ldflags "-X github.com/baggiiiie/configlock/cmd.version=1.0.0" -o configlock

# Build for specific OS
GOOS=linux GOARCH=amd64 go build -o configlock-linux
GOOS=darwin GOARCH=amd64 go build -o configlock-macos
```

### Dependencies

- `github.com/spf13/cobra` - CLI framework
- `github.com/fsnotify/fsnotify` - File system notifications
- `github.com/kardianos/service` - Cross-platform service management

## License

See the repository license file.

## Contributing

Contributions are welcome! Please ensure:
- Code follows the existing structure
- Changes align with the design documents in `docs/`
- Cross-platform compatibility is maintained

## Acknowledgments

Built as a productivity tool for developers who can't resist tinkering with their configs during work hours. You know who you are. 😉
