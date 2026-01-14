# Delete Email

This document describes the optimistic delete strategy used in both the Go TUI and Tauri desktop app.

## Overview

Maily uses an **optimistic delete** approach where emails are removed from the local cache immediately, providing instant UI feedback, while the actual IMAP operation is queued for background processing.

## Flow

```
┌─────────────────────────────────────────────────────────────────┐
│  1. User presses delete                                         │
│     ├─▶ Delete from SQLite cache immediately                   │
│     ├─▶ Queue IMAP operation (move to trash)                   │
│     └─▶ UI updates instantly                                   │
├─────────────────────────────────────────────────────────────────┤
│  2. Background worker processes queue                           │
│     ├─▶ Execute IMAP move-to-trash                              │
│     └─▶ On success: Delete from SQLite again                   │
│         (in case sync pulled email back)                        │
└─────────────────────────────────────────────────────────────────┘
```

## Why Optimistic Delete?

| Approach | User Experience | Consistency |
|----------|-----------------|-------------|
| **Synchronous** | Wait for IMAP (slow, can timeout) | Always consistent |
| **Optimistic** | Instant feedback | Eventually consistent |

Modern email clients (Gmail, Apple Mail, Outlook) all use optimistic updates because:
- IMAP operations can take 1-5+ seconds
- Network failures shouldn't block the UI
- Users expect instant feedback

## Implementation

### Go (TUI + Server)

**TUI side** (`internal/ui/commands.go`):
```go
func (a *App) moveSingleToTrash(uid imap.UID) tea.Cmd {
    return func() tea.Msg {
        // 1. Delete from local cache immediately
        diskCache.DeleteEmail(accountEmail, mailbox, uid)
        // 2. Queue for background processing
        diskCache.AddPendingOp(accountEmail, mailbox, cache.OpMoveTrash, uid)
        // 3. Return success immediately
        return singleDeleteCompleteMsg{uid: uid}
    }
}
```

**Server side** (`internal/server/state.go`):
- `pending_ops` SQLite table stores queued operations
- `ProcessPendingOps()` runs every 10 seconds
- Groups operations by account to reuse IMAP connections
- Deletes from cache again after successful IMAP operation

### Tauri (Desktop App)

**Rust side** (`src-tauri/src/lib.rs`):
```rust
fn delete_email(account: String, mailbox: String, uid: u32) -> Result<(), String> {
    // 1. Delete from local cache immediately
    delete_email_from_cache(&account, &mailbox, uid)?;
    // 2. Queue IMAP operation for background
    queue_move_to_trash(account, mailbox, uid);
    Ok(())
}
```

**Background worker** (`src-tauri/src/imap_queue.rs`):
- In-memory channel (`mpsc`) for queued operations
- Tokio async worker processes queue
- Deletes from cache again after successful IMAP operation

## Double Delete Safety

Both implementations delete from the cache twice:

1. **Immediately** when user initiates delete (for instant UI)
2. **After IMAP success** (in case sync pulled email back)

This prevents a race condition where:
1. User deletes email → removed from cache
2. Background sync runs → pulls email back from IMAP
3. IMAP delete completes → email still in cache!

The second delete ensures the email stays gone.

## Queue Persistence

| Platform | Queue Storage | Survives Crash? |
|----------|---------------|-----------------|
| Go | SQLite `pending_ops` table | Yes |
| Tauri | In-memory channel | No |

Go's SQLite-backed queue is more durable. Tauri could add SQLite persistence if needed.

## Error Handling

When IMAP operations fail:
- **Go**: Increments `retries` counter, stores `last_error` in `pending_ops`
- **Tauri**: Logs error to stderr, operation is lost

Future improvement: Surface persistent failures to the user (e.g., "3 operations pending" in status bar).

## Related Files

### Go
- `internal/cache/cache.go` - `pending_ops` table, `AddPendingOp()`, `GetPendingOps()`
- `internal/server/state.go` - `ProcessPendingOps()`
- `internal/server/server.go` - `backgroundPoller()` calls processor every 10s
- `internal/ui/commands.go` - `deleteSingleEmail()`, `moveSingleToTrash()`

### Tauri
- `src-tauri/src/imap_queue.rs` - `ImapOperation`, `queue_move_to_trash()`, worker
- `src-tauri/src/lib.rs` - `delete_email` command
- `src-tauri/src/mail.rs` - `delete_email_from_cache()`
