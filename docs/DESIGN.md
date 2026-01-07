# ConfigLock: A Productivity Enforcer for Config Files

## Overview

ConfigLock is a CLI tool and daemon written in Go that prevents editing of specified config files or directories during work hours (configurable, default 8:00 AM - 5:00 PM on weekdays). It uses system-level immutable flags to lock files, making it hard to bypass impulsively. The daemon runs in the background, periodically checking and re-applying locks. It's cross-platform (Linux and macOS).

### Key Features

- **CLI Commands**:
  - `configlock init`: Sets up the config file, prompts for work hours, installs and starts the daemon as a system service.
  - `configlock add <path>`: Adds a file or directory to the lock list (excludes `.git/` and `.jj/` in directories).
  - `configlock rm <path>`: Removes a file or directory from the lock list (requires typing challenge).
  - `configlock temp-unlock <path>`: Temporarily unlocks a file/directory for a configurable duration (default 5 min; requires typing challenge).
- **Daemon**: Background service that enforces locks during work hours, using file watchers for instant re-locking.
- **Typing Challenge**: For `rm` or `temp-unlock`, users must type a multi-line statement line-by-line with a typewriter effect to prevent copy-pasting.
- **Locking Mechanism**: Recursive immutable flags (`chattr +i -R` on Linux, `chflags schg -R` on macOS).
- **Config**: JSON file at `~/.config/configlock/config.json` for locked paths, work hours, and settings.

### Goals

- Make impulsive config tinkering during work hours frustrating and time-consuming.
- Allow full control outside work hours or via deliberate overrides.
- Ensure cross-platform compatibility with minimal dependencies.

### Installation

- Build from source: `go build -o configlock`.
- Run `sudo configlock init` to set up the daemon.

### Usage Examples

```
sudo configlock init
configlock add ~/.zshrc
configlock add ~/.config/nvim
configlock temp-unlock ~/.zshrc
configlock rm ~/.config/nvim
```

### Dependencies

- Go standard library.
- External: `github.com/fsnotify/fsnotify` (file watching), `github.com/kardianos/service` (service management).
