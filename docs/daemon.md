# Daemon

The maily daemon runs in the background to sync emails periodically.

## Commands

```bash
maily daemon start   # Start daemon (foreground, for debugging)
maily daemon status  # Check if daemon is running and view recent logs
maily daemon stop    # Stop the daemon
```

## How It Works

### Auto-Start

The daemon starts automatically when you launch `maily` (the TUI). You don't need to manually start it.

### Sync Interval

The daemon syncs all accounts every **30 minutes**. It syncs emails from the last 14 days for each account's INBOX.

### PID File

The daemon writes its PID and version to `~/.config/maily/daemon.pid` in the format:
```
PID:VERSION
```
Example: `12345:0.6.21`

This allows maily to detect version mismatches after upgrades.

### Log File

Logs are written to `~/.config/maily/daemon.log`. The log file is automatically rotated when it exceeds 10MB.

## Auto-Restart on Upgrade

### Homebrew upgrade

After running `brew upgrade maily`, the daemon is **not** automatically restarted. However, simply launching `maily` will detect the version mismatch and restart the daemon for you.

```bash
brew upgrade maily
maily   # Just launch maily - daemon auto-restarts
```

### maily update

If you installed maily manually (not via Homebrew), `maily update` handles everything automatically - it stops the daemon, updates the binary, and restarts the daemon.

### How auto-restart works:

1. When `maily` (TUI) launches, it calls `startDaemonBackground()`
2. This function reads the PID file to get the running daemon's version
3. If the versions don't match:
   - Sends `SIGTERM` to the old daemon
   - Waits 500ms for graceful shutdown
   - Starts a new daemon with the current binary

This means after `brew upgrade maily`, simply launching `maily` will automatically restart the daemon with the new version.

### Manual restart (if needed):

```bash
maily daemon stop
maily daemon start
```

## Graceful Shutdown

The daemon handles `SIGINT` and `SIGTERM` signals for graceful shutdown. When stopped:

1. Current sync operation completes
2. PID file is removed
3. Process exits cleanly
