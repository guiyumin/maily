# Email Cache Design

## Principle
**Server is the single source of truth.** Cache is just a local mirror.

## Architecture

```
┌─────────────────┐         ┌─────────────────┐
│   maily (TUI)   │ ←────── │   Local Cache   │
└─────────────────┘         └─────────────────┘
                                    ↑
                                    │ sync
                                    │
┌─────────────────┐         ┌─────────────────┐
│  maily daemon   │ ──────→ │   IMAP Server   │
└─────────────────┘         └─────────────────┘
     (every 30 min)
```

## Components

### 1. maily (TUI)
- Reads from local cache only
- Fast startup (no server wait)
- Can trigger manual sync: `r` to refresh
- First run (empty cache): auto-triggers sync, user waits once
- Auto-reloads after sync completes:
  - Watches metadata.json for changes (updated at end of sync)
  - Only reloads when no lock exists (ensures coherent snapshot)

### 2. maily daemon
- Runs in background
- Syncs every 30 minutes
- Fetches latest 14 days from server
- Saves to local cache
- Deletes cache files older than 14 days
- User starts with: `maily daemon start`
- User stops with: `maily daemon stop`

### 3. Manual refresh (`r` in TUI)
- Quick refresh: fetches latest 50 UIDs + FLAGS from server
- Compares with cache:
  - New UIDs → fetch full email
  - Changed flags → update cache
- Does NOT delete (can't distinguish "deleted" from "pushed out of top 50")
- Does NOT update older cached emails beyond the 50
- Faster than full sync
- Uses same lock as full sync
- If lock exists, shows "Sync in progress, try again later"

### 4. Full sync (`maily sync` from terminal)
- Same logic as daemon sync (full 14 days)
- Use when you need complete refresh

## Cache Structure

```
~/.config/maily/cache/
  user@gmail.com/
    INBOX/
      metadata.json    # {uidvalidity, last_sync}
      12345.json       # email by UID
      12346.json
    %5BGmail%5D%2FSent/
      ...
```

### Email JSON format
```json
{
  "uid": 12345,
  "internal_date": "2024-01-15T10:30:00Z",
  "from": "...",
  "to": "...",
  "subject": "...",
  "body": "...",
  "unread": true,
  "attachments": [...]
}
```

`internal_date` is used for ordering and 14-day cleanup.

## Sync Logic (daemon or manual)

**Timestamp**: Uses INTERNALDATE (server receive time), consistent across all operations.

```
1. Connect to IMAP
2. Check UIDVALIDITY (if changed, wipe cache, do full sync)
3. Fetch UID + FLAGS for last 14 days by INTERNALDATE (lightweight)
4. Compare with cached UIDs:
   - New UIDs on server → fetch full email, save to cache
   - UIDs in cache but not on server → delete from cache
   - Existing UIDs with changed flags → update cache
5. Delete cache files older than 14 days (by INTERNALDATE in JSON, not file mtime)
6. Update metadata.json with last_sync time
```

This handles:
- New emails (fetched)
- Deleted emails on other clients (removed from cache)
- Flag changes (read/unread synced)

### Atomic Writes
- Write to temp file first, then rename
- Prevents TUI from reading partial JSON during sync

### Sync Lock
- Lock file: `~/.config/maily/cache/<account>/.sync.lock`
- Contains PID of sync process
- Created when sync starts, deleted when sync finishes
- Per-account lock allows parallel sync of different accounts
- Prevents overlapping syncs for same account (daemon vs manual)
- Stale lock detection: if PID in lock file is not running, delete lock and proceed

## Delete Flow

```
User presses 'd'
  → Move to Trash on server (provider-specific)
  → Delete from cache
  → Update UI
```

### Trash Discovery
- Gmail: `[Gmail]/Trash`
- Others: Use IMAP `LIST` with `\Trash` special-use attribute
- Fallback: folder named `Trash`

## Search

- Always hits IMAP server directly
- Results not cached

## Email Fetch

Fetch these IMAP items per email:
- `INTERNALDATE` - server receive time (used for cleanup and ordering)
- `ENVELOPE` - headers (from, to, subject, date)
- `BODYSTRUCTURE` - MIME structure (attachment metadata: partID, filename, size, type)
- `BODY[TEXT]` - text content only, excludes attachment payloads

This keeps cache small while having attachment metadata.

## Attachments

- Cache metadata from BODYSTRUCTURE (filename, size, type, partID)
- Fetch content on demand when user saves (`BODY[<partID>]`)
- Decode Content-Transfer-Encoding (base64, quoted-printable) before saving
- Save to `~/Documents/maily/<account>/<filename>`

### Filename Sanitization
- Strip path separators (`/`, `\`)
- Replace invalid chars: `:`, `*`, `?`, `"`, `<`, `>`, `|`
- Limit to 255 chars
- Fallback: `attachment_<index>.bin` if invalid

## That's it.

No complex foreground/background sync.
No dedup logic.
No write queues.
Daemon handles everything.
