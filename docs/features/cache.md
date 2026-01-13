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
- Can trigger manual refresh: `R` (Shift+R) to fetch from server
- First run (empty cache): auto-triggers sync, user waits once
- Auto-reloads after sync completes:
  - Watches metadata.json for changes (updated at end of sync)
  - Only reloads when no lock exists (ensures coherent snapshot)

### 2. maily server
- Starts automatically when you open maily
- Runs in background, syncs every 30 minutes
- Uses `max(14 days, 100 emails)` sync strategy
- In-memory cache for instant TUI startup
- Also persists to disk for cold starts
- Check status: `maily server status`
- Stop: `maily server stop`

### 3. Manual refresh (`R` in TUI)
- Fetches emails from IMAP server directly
- Shows "Refreshing..." spinner while loading
- Updates UI with fresh data from server

### 4. Full sync (`maily sync` from terminal)
- Same logic as server sync: `max(14 days, 100 emails)`
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

## Sync Logic (server)

**Strategy**: `max(last 14 days, last 100 emails)`

This ensures:
- **Minimum coverage**: Always at least 100 emails (useful for low-activity accounts)
- **Recency coverage**: Never miss recent emails even during high-activity periods

```
1. Connect to IMAP
2. Fetch last 100 emails by sequence number (guaranteed minimum)
3. Fetch UIDs from last 14 days
4. Find any recent emails not in the 100 already fetched
5. Fetch those additional emails
6. Store union in memory cache and persist to disk
```

This handles:
- Low-activity accounts: always have 100 emails to browse
- High-activity accounts: never miss recent emails from last 14 days
- Flag changes synced on next refresh

### Atomic Writes
- Write to temp file first, then rename
- Prevents TUI from reading partial JSON during sync

### Sync Lock
- In-memory mutex per account (no file locks needed)
- Prevents overlapping syncs for same account
- Per-account lock allows parallel sync of different accounts

## Delete Flow

**No optimistic UI.** Wait for server confirmation before updating local state.

```
User presses 'd'
  → Show confirmation dialog
User presses 'y'
  → Show "Deleting..." spinner
  → Send delete request to IMAP server
  → Wait for server response
  → If success:
      → Delete from local cache
      → Remove from UI
      → Show "Successfully deleted"
  → If error (connection issue, etc.):
      → Show error message
      → Email stays in UI (no data loss)
```

This ensures consistency: if you see it deleted, it's deleted on server.

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

## Update Flow

When running `maily update`:

```
1. Stop server if running
2. Download and install new binary
3. Restart server if it was previously running
```

Safe because we use atomic writes - interrupted sync just means incomplete sync, next sync will fix it.

## That's it.

No complex foreground/background sync.
No dedup logic.
No write queues.
No optimistic UI - wait for server confirmation.
Server handles everything.
