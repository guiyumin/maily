# Bugfix: Stale Email Cache After External Deletion

**Date:** 2026-01-18
**Issue:** Emails deleted on another device (e.g., iPhone) still appear in Maily and show error when opened

## Symptoms

1. User deletes emails on their iPhone
2. Emails still appear in Maily's email list
3. When clicking on a deleted email, error appears: "Failed to load email content: email not found"
4. Emails persist until next sync cycle (up to 10 minutes)

## Investigation

### Sync Architecture

The codebase uses a client-server architecture:
- **Server** (`internal/server/`) manages IMAP connections and syncs every 10 minutes
- **Client** (TUI) connects via Unix socket and displays cached data
- **Cache** (`internal/cache/`) stores emails in SQLite at `~/.config/maily/maily.db`

### How Sync Detects Deleted Emails

In `internal/sync/sync.go:111-119`, the sync process compares cached UIDs with server UIDs:

```go
// Find deleted UIDs (in cache but not on server)
for uid := range cachedUIDs {
    if _, ok := serverUIDs[uid]; !ok {
        if err := s.cache.DeleteEmail(email, mailbox, uid); err != nil {
            continue
        }
    }
}
```

This works correctly during scheduled syncs, but there's a gap between syncs where stale emails can be viewed.

### The Error Path

When viewing an email:
1. `state.go:GetEmailWithBody()` checks cache for email body
2. If body is missing, fetches from IMAP via `imap.go:FetchEmailBody()`
3. IMAP returns 0 messages (email was deleted)
4. `imap.go:184` returns `fmt.Errorf("email not found")`
5. Error bubbles up to UI, displayed as-is

**Problem:** The error was shown to user but the stale email remained in cache and list.

## The Fix

### 1. Added Sentinel Error (`internal/mail/imap.go`)

```go
// ErrEmailNotFound is returned when an email no longer exists on the server
// (e.g., deleted from another device)
var ErrEmailNotFound = errors.New("email not found on server")
```

Changed line 184 to return this sentinel instead of a plain error string.

### 2. Cache Cleanup on Error (`internal/server/state.go`)

In `GetEmailWithBody()`, added handling for the sentinel error:

```go
if errors.Is(err, mail.ErrEmailNotFound) {
    sm.memory.Delete(email, mailbox, uid)
    if sm.cache != nil {
        _ = sm.cache.DeleteEmail(email, mailbox, uid)
    }
    return nil, fmt.Errorf("email was deleted on another device")
}
```

This ensures:
- Stale email is removed from memory cache
- Stale email is removed from disk cache (SQLite)
- User gets a meaningful error message

### 3. Graceful UI Handling (`internal/ui/app.go`)

In the `emailBodyErrorMsg` handler, added detection for the specific error:

```go
// Handle email deleted on another device - remove from list and go back
if strings.Contains(msg.err.Error(), "deleted on another device") {
    a.mailList.RemoveByUID(msg.uid)
    a.view = listView
    a.statusMsg = "Email was deleted on another device"
    return a, nil
}
```

This ensures:
- Email is removed from the visible list immediately
- User is returned to list view
- Friendly status message is shown

## Files Changed

| File | Change |
|------|--------|
| `internal/mail/imap.go` | Added `ErrEmailNotFound` sentinel error |
| `internal/server/state.go` | Delete stale email from cache when IMAP returns not found |
| `internal/ui/app.go` | Handle error gracefully, remove from list, show status |

## Testing

To test this fix:
1. Open Maily and view email list
2. Delete an email from another device (phone, webmail)
3. Click on the deleted email in Maily
4. **Expected:** Email disappears from list, status shows "Email was deleted on another device"
5. **Previous behavior:** Error message "Failed to load email content: email not found"

## Summary

The fix implements "lazy cleanup" - when a user happens to click on a stale email, the system:
1. Detects the email no longer exists on server
2. Cleans up the stale cache entry
3. Updates the UI gracefully
4. Informs the user what happened

This complements the existing 10-minute sync cycle, ensuring users always get a good experience even when viewing emails that were deleted elsewhere between sync cycles.
