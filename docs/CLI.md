# CLI Implementation Details

## Command Structure
Use `github.com/spf13/cobra` for CLI parsing (recommended) or the standard `flag` package.

### configlock init
- **Purpose**: Initialize the program.
- **Steps**:
  1. Create config directory `~/.config/configlock` if it doesn't exist.
  2. Prompt user for work start time (default "08:00"), end time (default "17:00"), and temp-unlock duration in minutes (default 5).
  3. Create `config.json` with default structure.
  4. Install the daemon as a user service using `github.com/kardianos/service`:
     - Linux: systemd user unit at `~/.config/systemd/user/configlock.service`
     - macOS: launchd agent at `~/Library/LaunchAgents/com.configlock.daemon.plist`
  5. Enable and start the service.
- **Output**: Clear success/error messages.

### configlock add <path>
- **Purpose**: Add file(s) or directory to the lock list.
- **Steps**:
  1. Resolve and validate absolute path.
  2. If directory: recursively collect all files (using `filepath.WalkDir`), skipping any paths containing `/.git/` or `/.jj/`.
  3. Optionally create `.bak` backups (configurable flag `--no-backup` to disable).
  4. Add absolute paths to `locked_paths` array in config (deduplicate).
  5. Save config.
  6. Immediately apply locks if currently within work hours.

### configlock rm <path>
- **Purpose**: Remove file(s)/directory from lock list.
- **Steps**:
  1. Perform typing challenge (see below).
  2. Remove matching paths from `locked_paths` (handle directory recursion as in `add`).
  3. Save config.
  4. Immediately unlock affected files/directories.

### configlock temp-unlock <path>
- **Purpose**: Temporarily disable locking for a path.
- **Steps**:
  1. Perform typing challenge.
  2. Unlock affected files/directories immediately.
  3. Add entry to `temp_excludes` map with expiration timestamp (current time + duration).
  4. Save config.
- **Flags**: `--duration <minutes>` to override default.

### Typing Challenge (for rm and temp-unlock)
- **Statement** (multi-line):
```
I UNDERSTAND THIS ACTION WILL DECREASE,
AND POTENTIALLY ELIMINATE, MY PRODUCTIVITY.
I UNDERSTAND THE RISK INVOLVED,
AND I AM WILLING TO PROCEED.
```

- **Implementation**:
1. Split into lines.
2. For each line:
   - Print the line character-by-character (typewriter effect) with `time.Sleep(30-50ms)` per char and flush stdout.
   - Prompt user to type it exactly.
   - Read input (trim newline, no trimming spaces).
   - Require exact match (case-sensitive).
   - Allow up to 3 retries per line; fail entire challenge after too many failures.
3. Only proceed if all lines are typed correctly.
- **Goal**: Prevent copy-paste and add deliberate friction.

### Additional Commands (Recommended)
- `configlock status`: Show current locked paths, lock state, and any active temp unlocks.
- `configlock list`: List all locked paths.

### General CLI Notes
- All commands load/save `~/.config/configlock/config.json`.
- Use absolute paths everywhere.
- Comprehensive error handling and user-friendly messages.
