# Security Fix: Init Bypass Vulnerability

## Problem
Previously, users could bypass the lock system by simply running `configlock init` again. This would:
1. Ask for a simple yes/no confirmation to overwrite the config
2. Create a fresh config file with an empty `locked_paths` list
3. Effectively remove all locked files from tracking without requiring the typing challenge

This was a critical security vulnerability that defeated the entire purpose of the tool.

## Solution
The fix implements two security measures:

### 1. Typing Challenge Requirement
When re-initializing and the config file is **locked** (during work hours):
- The user must complete the typing challenge before proceeding
- This prevents impulsive bypass attempts
- Maintains the same friction as `temp-unlock` command

When the config file is **not locked** (outside work hours):
- Simple yes/no confirmation is sufficient
- This is acceptable since locks aren't active anyway

### 2. Preserve Existing Locked Paths
When re-initializing:
- The existing `locked_paths` list is loaded from the current config
- After creating the new config, all previously locked paths are restored
- This ensures no files are "lost" from the lock list during re-initialization

## Code Changes

### File: `cmd/init.go`

**Added import:**
```go
"github.com/baggiiiie/configlock/internal/challenge"
```

**Modified config existence check (lines 38-75):**
- Check if config file is locked using `locker.IsLocked()`
- If locked, require typing challenge via `challenge.Run()`
- If not locked, use simple confirmation
- Load and preserve existing locked paths

**Modified config creation (lines 176-188):**
- Restore all existing locked paths after creating new config
- Skip duplicates (config path is already added)

## Testing the Fix

### Test Case 1: Re-init during work hours (config is locked)
```bash
$ configlock init
⚠️  Config file is currently locked.
Re-initializing will modify the configuration.
You must complete the typing challenge to proceed.

⚠️  WARNING: You are about to perform an action that may reduce your productivity.
To proceed, you must type the following statement line by line.
...
```

### Test Case 2: Re-init outside work hours (config not locked)
```bash
$ configlock init
Config file already exists. Overwrite? (y/N): y
Preserving 3 existing locked path(s)
...
```

### Test Case 3: Verify paths are preserved
```bash
$ configlock list
# Before re-init
~/.zshrc
~/.config/nvim
~/.gitconfig

$ configlock init
# ... complete re-init ...

$ configlock list
# After re-init - all paths still present
~/.zshrc
~/.config/nvim
~/.gitconfig
```

## Impact
This fix closes a critical security vulnerability that would have allowed users to easily bypass the lock system. The combination of typing challenge (during work hours) and path preservation ensures that re-initialization cannot be used as a backdoor to remove locks.
