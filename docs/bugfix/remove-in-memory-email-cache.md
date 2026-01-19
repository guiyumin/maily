# Bugfix: Remove In-Memory Email Cache

## Problem

1. **Unnecessary "Loading..." delays**: Constantly had to wait for emails to load, even though the SQLite cache database (`maily.db`) already had all the data. The in-memory email cache in the TUI was the culprit.

2. **Stale header rendering** (separate issue): When exiting from email read view back to list view, the "From: xxx@example.com" header persisted where the account tabs should be.

3. **Transient bug after removing cache**: After removing the in-memory cache, account switching showed "No emails to display" even though status bar showed "Inbox: 60 emails". This was a side effect that needed fixing.

## Root Causes

### Issue 1: Redundant In-Memory Cache

The TUI maintained an in-memory `emailCache map[string][]mail.Email` that duplicated data already stored in SQLite. This was unnecessary because:

- SQLite reads are fast (~milliseconds for hundreds of emails)
- The cache only helped within a single session after visiting an account
- After restart, the in-memory cache was empty, so it always showed "Loading..." even though SQLite had the data
- It added complexity: two caches to keep in sync instead of one source of truth

**Fix**: Removed `emailCache` entirely. TUI now reads directly from SQLite via server.

### Issue 2: Missing `tea.ClearScreen`

When transitioning from `readView` to `listView`, no `tea.ClearScreen` command was returned. Bubbletea's incremental rendering left old content on screen.

**Fix**: Added `return a, tea.ClearScreen` to all view transitions from readView/composeView back to listView.

### Issue 3: State Transition Bug (transient)

After removing the in-memory cache, account switching didn't trigger a full redraw because `state` never changed.

**Broken code:**
```go
a.mailList.SetEmails(nil)
return a, a.loadCachedEmails()  // state stayed as stateReady
```

The cachedEmailsLoadedMsg handler set emails and statusMsg correctly, but Bubbletea's optimized rendering didn't fully redraw the mailList because `state` never changed.

**Fixed code:**
```go
a.state = stateLoading
a.mailList.SetEmails(nil)
a.statusMsg = "Loading..."
return a, tea.Batch(a.spinner.Tick, a.loadCachedEmails())
```

Changing `state` from `stateLoading` → `stateReady` forces Bubbletea to switch between completely different views (spinner vs list), ensuring a full redraw.

## Changes Made

### TUI Changes

- `internal/ui/app.go`:
  - Removed `emailCache map[string][]mail.Email` field
  - Removed all `emailCache` read/write operations
  - Added `tea.ClearScreen` to readView → listView transitions
  - Added `stateLoading` during account switch

- `internal/ui/components/styles.go`:
  - No changes needed (HeaderStyle.MarginBottom(1) was investigated but not the cause)

### Server Changes

- `internal/server/state.go`:
  - Removed `memory *MemoryCache` field from `StateManager`
  - Simplified `GetEmails()` to read directly from SQLite cache
  - Simplified `CacheEmails()` to write directly to SQLite cache
  - Removed `NewMemoryCache()` initialization

- `internal/server/server.go`:
  - Removed `s.state.SetEmails()` call after sync

- `internal/server/memory.go`:
  - **Deleted entirely** - no more in-memory email storage

- `internal/cache/cache.go`:
  - Added `CountEmails()` method for email count queries

## Key Insight

**Bubbletea rendering optimization**: When returning from Update(), if the view structure doesn't fundamentally change (same `state`, same `view`), Bubbletea may do partial/optimized redraws. Changing `state` forces a complete view switch, ensuring all components render fresh.

## Architecture Simplification

Before:
```
IMAP → Server MemoryCache → SQLite → TUI emailCache → mailList
```

After:
```
IMAP → SQLite → mailList
```

All in-memory email caching has been removed:
- **Server**: Only caches IMAP connections (via `imapClient` in `AccountState`), no email data
- **TUI**: Reads directly from SQLite via server
- **SQLite**: Single source of truth for all cached email data

SQLite reads are fast enough (~milliseconds) that the extra cache layers were unnecessary complexity.

## Testing

1. Launch maily - emails should display
2. Press Tab to switch accounts - should show "Loading..." briefly, then emails
3. Open an email (Enter), then go back (Esc) - header should show account tabs, not "From:"
4. Quit and relaunch - should work the same as step 2
