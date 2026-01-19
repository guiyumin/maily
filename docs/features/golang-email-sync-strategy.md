# Email Sync Strategy

This document describes how Maily synchronizes emails between the IMAP server and local cache.

## Architecture Overview

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   IMAP Server   │────▶│  Memory Cache   │────▶│   Disk Cache    │
│  (Gmail, etc.)  │     │  (in-memory)    │     │   (SQLite)      │
└─────────────────┘     └─────────────────┘     └─────────────────┘
         │                      │                       │
         │                      ▼                       │
         │              ┌─────────────────┐             │
         └─────────────▶│    TUI Client   │◀────────────┘
                        └─────────────────┘
```

- **IMAP Server**: Source of truth for email data
- **Memory Cache**: Fast in-memory storage for active session
- **Disk Cache**: SQLite database for persistence across sessions

## Sync Triggers

| Trigger | Interval | Function |
|---------|----------|----------|
| Background poller | Every 10 minutes | `state.Sync()` |
| Manual refresh | `R` key in TUI | `quickRefresh()` |
| Server startup | On start (if cache stale) | `syncAllAccountsIfStale()` |

## Terminology

| Term | What it includes | IMAP fetch speed |
|------|------------------|------------------|
| **UID + Flags** | Just UID and read/unread status | Very fast, tiny response |
| **Metadata** | UID, Flags, Envelope (From, To, Subject, Date), InternalDate, attachment info | Fast, small response |
| **Full email** | Metadata + Body content (HTML/text) | Slow, large response |

In the sync algorithm:
- Step 1 fetches **Metadata** for 100 emails
- Step 2 fetches **UID + Flags** for 14-day window (lightweight, overlap with Step 1 is minimal)
- Step 3 fetches **Metadata** for missing emails only
- Step 7 fetches **Full email** for 10 most recent

## Sync Algorithm (`state.Sync`)

Located in `internal/server/state.go`.

### Sync Goal

Fetch the **union** of (last 100 emails) and (all emails from last 14 days):

```
Final emails = (last 100 by sequence) ∪ (all from last 14 days)
```

This ensures:
- **At least 100 emails** for users with sparse mail
- **Never miss recent emails** for busy inboxes where 100 emails might only cover a few days

| Scenario | Last 100 | Last 14 days | Result |
|----------|----------|--------------|--------|
| Light inbox | 100 emails (spans 2 months) | 30 emails | 100 emails |
| Busy inbox | 100 emails (spans 3 days) | 500 emails | 500 emails |
| Very light | 100 emails (spans 1 year) | 5 emails | 100 emails |

### Step 1: Fetch Metadata for Last 100 Emails

```go
emails, err := client.FetchMessagesMetadata(mailbox, 100)
```

Fetches **metadata only** (no body content) for the 100 most recent emails by sequence number. This is fast even on slow servers.

Metadata includes:
- UID, MessageID, InternalDate
- From, To, Subject, Date
- Flags (read/unread)
- Attachment info (not content)

### Step 2: Get UIDs from Last 14 Days

```go
since := time.Now().AddDate(0, 0, -14)
recentUIDs, err := client.FetchUIDsAndFlags(mailbox, since)
```

Fetches UIDs and flags for all emails in the last 14 days.

### Step 3: Fetch Missing Recent Emails

```go
for uid := range recentUIDs {
    if !fetchedUIDs[uid] {
        missingUIDs = append(missingUIDs, uid)
    }
}
additional, err := client.FetchMessagesByUIDsMetadata(mailbox, missingUIDs)
```

Fetches metadata for any emails in the 14-day window not already in the first 100. This completes the union.

### Step 4: Update Memory Cache

```go
sm.SetEmails(email, mailbox, cached)
```

Replaces the entire memory cache for this mailbox. This automatically removes stale emails from memory.

### Step 5: Persist to Disk Cache

```go
for _, c := range cached {
    sm.cache.InsertEmailMetadataIfMissing(email, mailbox, c)
}
```

Inserts new emails to disk cache. Existing emails are not overwritten (preserves body content).

### Step 6: Remove Stale Emails from Disk

```go
cachedUIDs, _ := sm.cache.GetCachedUIDs(email, mailbox)
for uid := range cachedUIDs {
    if !serverUIDs[uid] {
        sm.cache.DeleteEmail(email, mailbox, uid)
    }
}
```

Compares cached UIDs against server UIDs. Any email in the cache but not on the server is deleted. This handles emails deleted on other devices (iPhone, web, etc.).

### Step 7: Prefetch Body for 10 Most Recent

```go
for i := 0; i < len(cached) && len(prefetchUIDs) < 10; i++ {
    if cached[i].BodyHTML == "" {
        prefetchUIDs = append(prefetchUIDs, cached[i].UID)
    }
}
fullEmails, _ := client.FetchMessagesByUIDs(mailbox, prefetchUIDs)
```

Fetches full body content for the 10 most recent emails that don't have body cached. This ensures the most recent emails are immediately readable without additional IMAP fetches.

### Step 8: Update Metadata

```go
sm.cache.SaveMetadata(email, mailbox, &cache.Metadata{
    UIDValidity: uidValidity,
    LastSync:    time.Now(),
})
```

Saves sync timestamp and UIDVALIDITY for cache freshness checks.

## Body Fetching Strategy

Bodies are fetched using a hybrid approach:

1. **Prefetch**: 10 most recent emails have body fetched during sync
2. **Lazy load**: Other emails fetch body on-demand when user opens them
3. **Aggressive caching**: Once fetched, body is persisted to disk cache via `UpdateEmailBody()`

This balances fast sync times with good UX for recent emails.

## UIDVALIDITY Handling

IMAP servers assign a UIDVALIDITY value to each mailbox. If this value changes, all cached UIDs are invalid.

```go
if meta.UIDValidity != info.UIDValidity {
    sm.cache.InvalidateMailbox(email, mailbox)
}
```

When UIDVALIDITY changes, the entire mailbox cache is cleared and rebuilt.

## Cache Retention

- **Time-based**: Emails older than 14 days are cleaned up
- **Count-based**: At least 100 most recent emails are always kept

## Concurrency

- **Per-account locking**: `TryStartSync()` prevents concurrent syncs for the same account
- **Memory cache**: Uses `sync.RWMutex` for thread-safe access
- **Disk cache**: SQLite handles concurrent writes

## Quick Refresh vs Full Sync

| Aspect | Quick Refresh (`R` key) | Full Sync (background) |
|--------|------------------------|------------------------|
| Emails fetched | Last 100 | Last 100 + 14-day window |
| Stale removal | Yes | Yes |
| Body prefetch | Yes (10 most recent) | Yes (10 most recent) |
| Metadata update | Yes | Yes |

## Data Flow Diagram

```
User opens Maily
        │
        ▼
┌───────────────────┐
│ Load from disk    │──▶ Fast startup with cached data
│ cache (SQLite)    │
└───────────────────┘
        │
        ▼
┌───────────────────┐
│ Connect to server │──▶ Start background poller
└───────────────────┘
        │
        ▼
┌───────────────────┐
│ Sync if stale     │──▶ Skip if cache is fresh (<10 min)
└───────────────────┘
        │
        ▼
┌───────────────────┐
│ Every 10 minutes  │──▶ Background sync all accounts
└───────────────────┘
```

## Key Files

| File | Purpose |
|------|---------|
| `internal/server/state.go` | `Sync()` function, state management |
| `internal/server/memory.go` | In-memory cache implementation |
| `internal/cache/cache.go` | SQLite disk cache |
| `internal/mail/imap.go` | IMAP client, fetch methods |
| `internal/sync/sync.go` | Legacy sync (used by CLI) |
