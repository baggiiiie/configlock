# ConfigLock

A CLI tool that locks config files during work hours using system-level immutable flags. Prevents impulsive config tinkering when you should be working.

## Features

- Uses system-level immutable flags
- Configurable work hours with simple time ranges or cron expressions
- File system watcher detects and re-locks files immediately if modified
- Typing challenge for unlock operations to prevent impulsive actions
- Temporary unlocks with configurable durations
- Runs as a background daemon
- Supports Linux and macOS

## Installation

### Homebrew (Recommended)

```bash
brew install baggiiiie/tap/configlock
```

### Curl

```bash
curl -sSL https://raw.githubusercontent.com/baggiiiie/configlock/main/install.sh | bash
```

## Usage

### Setup

```bash
configlock init
```

This creates the config directory, prompts for work hours and temp unlock duration, installs the daemon, and locks the config file itself.

Time input formats:

- Simple: `HH:MM` or `HHMM` (e.g., `14:30` or `1430`)
- Cron: `cron:MIN HOUR DAY MONTH WEEKDAY` (e.g., `cron:0 8-17 * * 1-5`)

### Commands

```bash
# Add files or directories to lock list
configlock add ~/.zshrc
configlock add ~/.config/nvim

# List locked paths
configlock list

# View current status
configlock status

# Edit work hours
configlock edit

# Temporarily unlock a path (requires typing challenge)
configlock temp-unlock ~/.zshrc
configlock temp-unlock ~/.zshrc --duration 10

# Remove from lock list
configlock rm ~/.config/nvim

# Daemon control
configlock start
configlock stop
```

## Configuration

Config file: `~/.config/configlock/config.json`

```json
{
  "locked_paths": ["/home/user/.zshrc"],
  "start_time": "08:00",
  "end_time": "17:00",
  "cron_schedule": "",
  "temp_duration": 5,
  "temp_excludes": {}
}
```

## Troubleshooting

## Uninstalling

```bash
# Linux
systemctl --user stop configlock
systemctl --user disable configlock
rm ~/.config/systemd/user/configlock.service
systemctl --user daemon-reload

# macOS
launchctl unload ~/Library/LaunchAgents/com.configlock.daemon.plist
rm ~/Library/LaunchAgents/com.configlock.daemon.plist

# Both
rm -rf ~/.config/configlock
rm /usr/local/bin/configlock
```

### Force unlock files

```bash
# Linux
chattr -i -R /path/to/file

# macOS
chflags -R noschg /path/to/file
```

### Check daemon status

```bash
# Linux
systemctl --user status configlock

# macOS
launchctl list | grep configlock
```

## License

MIT
