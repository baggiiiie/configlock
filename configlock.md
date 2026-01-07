# ConfigLock (Go) — Implementation Details

This document describes the **design and implementation details** for a cross-platform (macOS + Linux) tool called **ConfigLock**.

The goal is to **prevent editing of configuration files during work hours** using **Randomized Permission Obfuscation (RPO)**, while keeping tools (e.g. Neovim) fully functional for reading configs.

This spec is intended to be handed directly to coding agents.

---

## 1. Design Goals

**Primary goals**
- Cross-platform: macOS + Linux
- Simple primitives (POSIX permissions only)
- No sudo required
- Configs remain readable
- Writing is blocked
- Reversal is intentionally annoying (but safe)

**Non-goals**
- Strong security against a determined admin
- Kernel-level enforcement
- Preventing deliberate sabotage by the user

This is a *friction system*, not DRM.

---

## 2. Core Concept: Randomized Permission Obfuscation (RPO)

Instead of simply removing write permissions (`chmod -w`), ConfigLock:

1. Records the exact original permissions
2. Applies a **randomized permission mask** that:
   - Always disables write
   - Always preserves read (and execute for dirs)
3. Hides the restore metadata
4. Restores permissions *exactly* after the lock window

This destroys "undo muscle memory" while keeping tools functional.

---

## 3. Scope of Locking

### Supported Targets
- Individual files (e.g. `~/.zshrc`)
- Directory trees (e.g. `~/.config/nvim`)

### Explicitly Required Behavior
- Neovim must still be able to:
  - Read config files
  - Traverse config directories
- No writes allowed inside locked paths

---

## 4. Permission Rules (Critical)

### Files
- **Must keep**: read (`r`)
- **Must remove**: write (`w`)
- Execute bit may be random

Allowed file permissions:
- `400`, `440`, `444`, etc.

### Directories
- **Must keep**: read + execute (`r-x`)
- **Must remove**: write (`w`)

Allowed directory permissions:
- `500`, `510`, `550`, `555`, etc.

Never remove execute from directories.

---

## 5. Permission Randomization Strategy

### Files

Generate permissions in the range:
```
400–447 (octal)
```

This guarantees:
- Owner read
- No owner write
- Randomized group/other bits

### Directories

Generate permissions in the range:
```
500–557 (octal)
```

This guarantees:
- Traversable
- Readable
- Not writable

---

## 6. Metadata Snapshot

Before locking, the program must record:

For each path:
- Absolute path
- Original permission bits (octal)
- File type (file or directory)

Optional (but recommended):
- SHA-256 hash of file contents (tamper detection)

### Storage Requirements
- Stored in `$HOME/.configlock/`
- Filename must be **randomized** (not predictable)
- No human-readable filenames like `perms.json`

---

## 7. Snapshot Obfuscation

### Minimum Requirement
- Snapshot file name is random (UUID or random hex)

### Optional Hardening
- XOR or encrypt snapshot with a time-derived key
- Delay ability to decrypt until unlock window

The goal is **not cryptographic security**, but to prevent casual recovery.

---

## 8. Lock Flow

### `configlock lock`

1. Resolve all target paths
2. Walk directory trees recursively
3. For each file/dir:
   - Record original permissions
   - Apply randomized non-writable permissions
4. Store snapshot metadata
5. Write a `lock_state` marker file

Lock must be **idempotent**.

---

## 9. Unlock Flow

### `configlock unlock`

1. Verify unlock is allowed (time-based check optional)
2. Locate snapshot metadata
3. Restore exact original permissions
4. Remove snapshot + state markers

Unlock must fail safely if snapshot is missing.

---

## 10. Tamper Detection (Optional but Recommended)

On `lock` or periodic checks:
- If permissions differ from expected randomized state:
  - Re-randomize
  - Optionally escalate (harsher perms)

This discourages manual chmod attempts.

---

## 11. CLI Interface

ConfigLock is an **interactive, opinionated CLI tool**.

### Supported Commands

```
configlock add <file|directory>
configlock rm <file|directory>
configlock temp-unlock <file|directory>
configlock lock
configlock unlock
configlock status
```

#### `add <path>`
- Registers a file or directory to be managed by ConfigLock
- Immediately applies Randomized Permission Obfuscation if currently in a locked state
- Paths are stored as absolute paths

#### `rm <path>`
- Permanently removes a path from ConfigLock management
- Restores original permissions
- Requires **explicit acknowledgment ritual** (see Section 12)

#### `temp-unlock <path>`
- Temporarily restores original permissions for a single path
- Does **not** affect other locked paths
- Automatically re-locks after the next global lock cycle
- Requires **explicit acknowledgment ritual** (see Section 12)

#### `lock`
- Applies locking to all registered paths

#### `unlock`
- Restores permissions for all registered paths

#### `status`
- Shows registered paths and current lock state

---


## 12. Explicit Acknowledgment Ritual

Certain actions are intentionally painful and slow.

The following commands require a **manual acknowledgment ritual**:
- `configlock rm`
- `configlock temp-unlock`

### Required Statement

The user must type the following statement **exactly**, without copy-and-paste:

```
I UNDERSTAND THIS ACTION WILL DECREASE, AND POTENTIALLY ELIMINATE, MY PRODUCTIVITY.
I UNDERSTAND THE RISK INVOLVED, AND I AM WILLING TO PROCEED
```

### Presentation Rules

- The statement is **not shown all at once**
- It is revealed **line by line**
- Each line is rendered using a **typewriter effect**
- The next line is shown **only after** the previous line has been typed correctly

### Input Rules

- Pasting input must be rejected
- Input must be read from a TTY
- Backspace is allowed
- Any mismatch resets the current line

### Failure Behavior

- On mismatch: re-render the same line
- On Ctrl+C: abort without changes
- On timeout (optional): abort

This ritual is intended to defeat muscle memory and impulse overrides.

---

## 13. Configuration

### Managed Paths Registry

- Stored in `$HOME/.configlock/paths.json`
- Each entry:
  - Absolute path
  - Type (file or directory)
  - Snapshot metadata reference

Hardcoded defaults may be pre-registered (e.g. `~/.zshrc`).

---


## 13. Cross-Platform Considerations

### Permissions
- Use POSIX permission bits only
- Avoid platform-specific flags (`chattr`)

### Stat APIs
- Use Go `os.Stat` / `FileMode`
- Always operate on octal permissions

---

## 14. Scheduling (Out of Scope for Core)

Lock/unlock scheduling should be handled externally:

- macOS: `launchd`
- Linux: `cron` or `systemd timer`

This keeps the binary simple.

---

## 15. Safety Guarantees

The program must guarantee:
- No data loss
- No unreadable config files
- No removal of execute bits on directories

---

## 16. Recommended Directory Layout

```
$HOME/.configlock/
  ├── .state
  ├── .snapshot_<random>
```

All files should be hidden.

---

## 17. Development Philosophy

- Opinionated > configurable
- Friction > flexibility
- Predictability for tools, unpredictability for humans

This tool exists to protect the user's focus, not their system.

---

## 18. Summary

ConfigLock:
- Uses Go
- Uses POSIX permissions only
- Applies constrained randomization
- Preserves tool functionality
- Breaks undo muscle memory

This document is sufficient to implement the full MVP.

