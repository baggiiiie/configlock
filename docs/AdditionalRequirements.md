# Additional Requirements & Recommendations

## Security & Robustness

- Daemon must run as root to apply system immutable flags.
- Protect the config file itself: add `~/.config/configlock/config.json` to the lock list automatically after init.
- Validate and sanitize all paths (config files might be symlink. for example, `~/.config/nvim/` points to `~/Desktop/repos/myconfig/nvim`).
- File locking on config.json when reading/writing to prevent corruption.

## Logging

- Structured logs (timestamp, level, message).
- Rotate logs when >10MB.

## Nice-to-Have Features

- `configlock status`: Pretty-print current state.
- Desktop notification on blocked attempt (using `notify-send` on Linux, `osascript` on macOS).
- Dry-run mode for add/rm.
- Configurable statement text (stored in config.json).

## Build & Distribution

- Single static binary (no cgo dependencies).
- Include version flag (`-v`).
