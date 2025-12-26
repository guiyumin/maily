# Bug: Stale Daemon After Homebrew Upgrade

## Problem

After running `brew upgrade maily`, the daemon continues running with old code.

When Homebrew upgrades maily:
1. The binary at `/opt/homebrew/bin/maily` is replaced with the new version
2. The old daemon process is still running in memory with old code
3. When user runs `maily`, it checks if daemon is running via PID file
4. Since a process named "maily" is running, it assumes daemon is fine
5. The old daemon continues running with stale code indefinitely

New bug fixes or features in the daemon won't take effect until user manually restarts the daemon or reboots.

## Reproduction

```sh
# Before fix: daemon keeps running with old code after upgrade
brew upgrade maily
maily daemon status
# Shows: Daemon is running (PID: 12345)
# But it's running OLD code!
```

## Solution

Store version in PID file and auto-restart daemon on version mismatch.

### Changes to `internal/cli/daemon.go`

1. **PID file format**: Changed from `PID` to `PID:VERSION`
   ```
   # Old format
   12345

   # New format
   12345:0.6.5
   ```

2. **New `parsePidFile()` function**: Reads PID and version from PID file, with backwards compatibility for old format.

3. **Updated `startDaemonBackground()`**: Checks if running daemon version matches current binary. If mismatch, stops old daemon and starts new one.

4. **Updated `checkDaemonStatus()`**: Now displays daemon version.

## Verification

```sh
# After fix: daemon auto-restarts with new version
maily daemon status
# Daemon is running (PID: 85002, version: v0.6.5)

# Simulate version mismatch (or brew upgrade)
echo "85002:0.5.0" > ~/.config/maily/daemon.pid

maily  # Just running maily triggers restart
maily daemon status
# Daemon is running (PID: 85151, version: v0.6.5)  # New PID!
```

## Backwards Compatibility

If the PID file has no version (old daemon from before this fix), it will be restarted to get the new version-aware daemon.
