# Tauri Email Sync Strategy

This document describes how the Tauri desktop app synchronizes emails between IMAP servers and the local SQLite cache.

## Quick Overview

**Sync is triggered by:**
- **Background timer**: Every 10 minutes (starts 1 minute after app launch)
- **Manual refresh**: User clicks refresh button or presses R
- **App startup**: If cache is empty

**Two-phase approach:**
1. **Metadata sync** - Download ENVELOPE only (~100 bytes per email)
2. **Body prefetch** - Download body for 10 most recent emails
3. **Lazy load** - Other bodies fetched when user opens email

---

## Step-by-Step Sync Algorithm

### Step 1: Fetch last 100 emails by sequence
```
IMAP: FETCH 1:* (UID FLAGS INTERNALDATE)
```
Gets UIDs, read/unread flags, and dates for the most recent 100 emails. No body downloaded.

### Step 2: Search for emails from last 14 days
```
IMAP: UID SEARCH SINCE 04-Jan-2026
```
Returns UIDs of all emails in the 14-day window.

### Step 3: Find missing emails
Compares Step 1 and Step 2 results. If any emails from the 14-day window weren't in the last 100, fetch their UIDs and flags too.

### Step 4: Download ENVELOPE metadata (FAST)
```
IMAP: UID FETCH 123,456,789 (UID FLAGS INTERNALDATE ENVELOPE)
```
For **new** emails only (not in cache), downloads:
- From, To, CC, Reply-To
- Subject, Date, Message-ID
- **No body** (~100 bytes per email vs ~50KB for full email)

Saves to SQLite with empty `body_html`.

### Step 5: Update flags for existing emails
For emails already in cache, checks if read/unread status changed on server and updates cache.

### Step 6: Remove stale emails
Deletes emails from cache that no longer exist on server (user deleted them from phone/web).

### Step 7: Prefetch body for 10 most recent
```
IMAP: UID FETCH 123,124,125... (UID RFC822)
```
Downloads full body for the 10 most recent emails without body in cache. This ensures recent emails are immediately readable.

### Step 8: Save mailbox metadata
Stores UIDVALIDITY and LastSync timestamp.

---

## When User Opens an Email

```
Frontend: invoke("get_email_with_body", { account, mailbox, uid })
```

**Backend logic:**
1. Load email from SQLite cache
2. If `body_html` is empty:
   - Connect to IMAP
   - Fetch RFC822 body
   - Parse HTML body and snippet
   - Update cache
3. Return complete email

---

## Visual Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                         SYNC PHASE                              │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Step 1-3: Determine which emails to sync                       │
│            (UIDs + FLAGS only, very fast)                       │
│                        │                                        │
│                        ▼                                        │
│  Step 4: Fetch ENVELOPE for new emails                          │
│          ┌────────────────────────────────┐                     │
│          │ From: alice@example.com        │  ~100 bytes         │
│          │ Subject: Hello                 │  per email          │
│          │ Date: Jan 18, 2026             │                     │
│          │ Body: (empty)                  │                     │
│          └────────────────────────────────┘                     │
│                        │                                        │
│                        ▼                                        │
│  Step 5-6: Update flags, remove deleted emails                  │
│                        │                                        │
│                        ▼                                        │
│  Step 7: Prefetch body for 10 most recent                       │
│          ┌────────────────────────────────┐                     │
│          │ Body: <html>Full content...</  │  ~50KB              │
│          │       </html>                  │  per email          │
│          └────────────────────────────────┘                     │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘

┌─────────────────────────────────────────────────────────────────┐
│                      USER OPENS EMAIL                           │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  User clicks email #50 (not in top 10)                          │
│                        │                                        │
│                        ▼                                        │
│  get_email_with_body(account, mailbox, uid=50)                  │
│                        │                                        │
│                        ▼                                        │
│  ┌─────────────────────────────────────┐                        │
│  │ Body in cache?                      │                        │
│  └─────────────────────────────────────┘                        │
│         │                    │                                  │
│        YES                  NO                                  │
│         │                    │                                  │
│         ▼                    ▼                                  │
│    Return email       Fetch RFC822 from IMAP                    │
│                              │                                  │
│                              ▼                                  │
│                        Update cache                             │
│                              │                                  │
│                              ▼                                  │
│                        Return email                             │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

## Performance Benefit

| Scenario | Old (RFC822) | New (ENVELOPE + Lazy) |
|----------|--------------|----------------------|
| Sync 100 new emails | ~5MB download | ~10KB metadata + 500KB for 10 bodies |
| Time to see email list | Wait for all bodies | Immediate (metadata only) |
| Open old email | Already cached | ~50KB on-demand fetch |

---

## Architecture Overview

```
┌─────────────────┐     Tauri IPC      ┌─────────────────────────────────┐
│  React Frontend │ ◄────────────────► │         Rust Backend            │
│  (src/)         │                    │         (src-tauri/)            │
└─────────────────┘                    │                                 │
                                       │  ┌───────────────────────────┐  │
                                       │  │    Connection Pool        │  │
                                       │  │  (1 session per account)  │  │
                                       │  └───────────────────────────┘  │
                                       │            │                    │
                                       │            ▼                    │
                                       │  ┌───────────────────────────┐  │
                                       │  │      IMAP Queue           │  │
                                       │  │  (per-account workers)    │  │
                                       │  └───────────────────────────┘  │
                                       │            │                    │
                                       │            ▼                    │
                                       │  ┌───────────────────────────┐  │
                                       │  │     SQLite Cache          │  │
                                       │  │   (~/.config/maily/       │  │
                                       │  │        maily.db)          │  │
                                       │  └───────────────────────────┘  │
                                       └─────────────────────────────────┘
                                                    │
                                                    ▼
                                       ┌─────────────────────────────────┐
                                       │         IMAP Server             │
                                       │       (Gmail, Yahoo, etc.)      │
                                       └─────────────────────────────────┘
```

## Connection Pool

Located in `src-tauri/src/imap_queue.rs`.

### Design

Each email account maintains **one pooled IMAP connection** that is reused across operations:

```rust
struct PooledConnection {
    session: Option<Session<TlsStream<TcpStream>>>,
    last_used: Instant,
}

static CONNECTION_POOL: Lazy<Mutex<HashMap<String, Arc<Mutex<PooledConnection>>>>>
```

### Connection Lifecycle

| State | Behavior |
|-------|----------|
| **Missing** | Create new connection on first use |
| **Fresh** (< 5 min idle) | Reuse existing connection |
| **Stale** (> 5 min idle) | Close and create new connection |
| **Error** | Invalidate and reconnect on next use |

### `with_imap_connection()` Pattern

All IMAP operations use this function (equivalent to Go's `withIMAPClient`):

```rust
fn with_imap_connection<F, R>(
    account_name: &str,
    mailbox: &str,
    op: F,
) -> Result<R, Box<dyn std::error::Error + Send + Sync>>
```

Benefits:
- Automatic connection reuse
- Auto-reconnect on stale connections
- Error-based invalidation
- Mutex-guarded thread safety

## Sync Constants

```rust
const MIN_SYNC_EMAILS: usize = 100;     // Minimum emails to sync
const SYNC_DAYS: i64 = 14;              // Days to look back
const PREFETCH_BODY_COUNT: usize = 10;  // Bodies to prefetch
```

### Sync Goal

```
Final emails = (last 100 by sequence) ∪ (all from last 14 days)
```

| Scenario | Last 100 | Last 14 days | Result |
|----------|----------|--------------|--------|
| Light inbox | 100 (spans 2 months) | 30 | 100 emails |
| Busy inbox | 100 (spans 3 days) | 500 | 500 emails |
| Very light | 100 (spans 1 year) | 5 | 100 emails |

## Sync Triggers

| Trigger | Interval | Location |
|---------|----------|----------|
| **Background timer** | Every 10 minutes | `lib.rs` setup() |
| **Manual refresh** | User action (R key / button) | Frontend → `start_sync` |
| **App startup** | If cache empty | Frontend auto-sync |

### Background Sync Timer

```rust
// In lib.rs setup()
tokio::spawn(async {
    // Wait 1 minute before first sync
    tokio::time::sleep(Duration::from_secs(60)).await;

    let mut interval = tokio::time::interval(Duration::from_secs(600)); // 10 min
    loop {
        interval.tick().await;
        for account in get_accounts() {
            queue_sync(account.name, "INBOX".to_string());
        }
    }
});
```

## Operation Types

### 1. Async Operations

Return immediately, emit events when done.

| Operation | Function | Events |
|-----------|----------|--------|
| Sync | `queue_sync()` | `sync-started`, `sync-complete`, `sync-error` |

### 2. Queued Operations

Update cache immediately, IMAP operation in background.

| Operation | Function | Behavior |
|-----------|----------|----------|
| Delete | `queue_delete()` | Remove from cache → queue IMAP delete |
| Move to Trash | `queue_move_to_trash()` | Remove from cache → queue IMAP move |
| Mark Read | `queue_mark_read()` | Update cache → queue IMAP flag change |

### 3. Synchronous Operations

Block until IMAP responds (for operations requiring confirmation).

| Operation | Function | Use Case |
|-----------|----------|----------|
| Mark Read (sync) | `update_email_read_status()` | When confirmation needed |

## IMAP Queue

Located in `src-tauri/src/imap_queue.rs`.

### Per-Account Workers

Each account has a dedicated async worker:

```rust
static ACCOUNT_QUEUES: Lazy<Mutex<HashMap<String, Sender<ImapOperation>>>>
```

### Operation Batching

Operations within 100ms are batched together:

```
t=0ms:   User marks email 1 read
t=20ms:  User marks email 2 read
t=50ms:  User marks email 1 unread (overwrites earlier)
t=150ms: Timeout → Process batch
         - Email 1: mark unread
         - Email 2: mark read
```

### Operation Types

```rust
enum ImapOperation {
    MarkRead { mailbox, uid, unread },
    Delete { mailbox, uid },
    MoveToTrash { mailbox, uid },
    SyncMailbox { mailbox },
}
```

## Cache Architecture

### SQLite Schema

```sql
-- Email metadata and content
CREATE TABLE emails (
    account TEXT NOT NULL,
    mailbox TEXT NOT NULL,
    uid INTEGER NOT NULL,
    message_id TEXT,
    internal_date INTEGER,
    from_addr TEXT,
    to_addr TEXT,
    cc TEXT,
    subject TEXT,
    snippet TEXT,
    body_html TEXT,        -- Empty until body is fetched
    unread INTEGER,
    PRIMARY KEY (account, mailbox, uid)
);

-- Mailbox sync metadata
CREATE TABLE mailbox_metadata (
    account TEXT NOT NULL,
    mailbox TEXT NOT NULL,
    uid_validity INTEGER,
    last_sync INTEGER,
    PRIMARY KEY (account, mailbox)
);

-- Attachments
CREATE TABLE attachments (
    account TEXT, mailbox TEXT, email_uid INTEGER,
    part_id TEXT, filename TEXT, content_type TEXT,
    size INTEGER, encoding TEXT,
    PRIMARY KEY (account, mailbox, email_uid, part_id)
);
```

### Cache Operations

| Operation | Function | Behavior |
|-----------|----------|----------|
| Insert metadata | `save_email_metadata_to_db()` | Inserts ENVELOPE data, empty body |
| Insert full email | `save_email_to_db()` | Inserts full email with body |
| Update body | `update_email_body_in_db()` | Updates body_html + snippet |
| Get with lazy load | `get_email_with_body()` | Fetches body from IMAP if missing |
| Delete stale | `delete_stale_emails()` | Removes UIDs not on server |
| Get cached UIDs | `get_cached_uids()` | Returns all UIDs in cache |
| Get UIDs without body | `get_uids_without_body()` | For prefetch targeting |

## Stale Email Removal

Emails deleted on other devices (phone, web) are removed from cache:

```rust
fn delete_stale_emails(conn, account, mailbox, server_uids) -> usize {
    let cached_uids = get_cached_uids(conn, account, mailbox);
    for uid in cached_uids {
        if !server_uids.contains(&uid) {
            // Email was deleted on server, remove from cache
            conn.execute("DELETE FROM emails WHERE uid = ?", uid);
        }
    }
}
```

## Body Fetching Strategy

### Prefetch (During Sync)

10 most recent emails have body fetched during sync:

```rust
let prefetch_uids = get_uids_without_body(conn, account, mailbox, PREFETCH_BODY_COUNT);
for uid in prefetch_uids {
    // Fetch RFC822 and update body_html + snippet in cache
    session.uid_fetch(&uid_set, "(UID RFC822)")
}
```

### Lazy Load (On Demand)

Other emails fetch body when user opens them via `get_email_with_body()`:

```rust
pub fn get_email_with_body(account, mailbox, uid) -> Email {
    let email = get_email(account, mailbox, uid)?;  // From cache

    if email.body_html.is_empty() {
        // Connect to IMAP, fetch RFC822, parse body
        // Update cache with body_html and snippet
    }

    return email;
}
```

Frontend calls this when opening an email:

```typescript
// EmailReader.tsx
invoke<EmailFull>("get_email_with_body", { account, mailbox, uid })
```

## Event Flow

```
Frontend                    Backend                     IMAP Server
   │                           │                            │
   │─── start_sync ───────────►│                            │
   │                           │─── queue_sync() ──────────►│
   │◄── sync-started ─────────│                            │
   │                           │                            │
   │                           │◄── fetch ENVELOPE ────────│
   │                           │─── save metadata ─────────►│
   │                           │                            │
   │                           │◄── fetch RFC822 (10) ─────│
   │                           │─── update body ───────────►│
   │                           │                            │
   │◄── sync-complete ────────│                            │
   │    {new, updated,         │                            │
   │     deleted, total}       │                            │
```

## Key Files

| File | Purpose |
|------|---------|
| `src-tauri/src/imap_queue.rs` | Connection pool, operation queue, workers |
| `src-tauri/src/mail.rs` | Sync algorithm, cache operations, IMAP helpers |
| `src-tauri/src/lib.rs` | Background sync timer, Tauri command handlers |
| `src/components/home/Home.tsx` | Frontend sync event listeners |
| `src/components/home/EmailReader.tsx` | Lazy body loading via `get_email_with_body` |

## Comparison with Go TUI

| Aspect | Go TUI | Tauri Desktop |
|--------|--------|---------------|
| Process model | Separate server + client | Single process |
| Connection pool | Server-side, Unix socket | In-process, direct |
| Sync algorithm | Same | Same (100 + 14 days) |
| Metadata fetch | ENVELOPE (metadata only) | ENVELOPE (metadata only) |
| Body strategy | Lazy load on open | Prefetch 10 + lazy load |
| Background sync | 10-min poller in server | 10-min tokio timer |
| Operation queue | Server-side pending_ops | Per-account mpsc channels |
| Cache | Memory + Disk | Disk (SQLite) |

## Dependencies

The sync implementation uses:
- `imap` crate (v2) - IMAP protocol
- `imap-proto` crate (v0.10) - For `Envelope` and `Address` types used in metadata parsing
- `mailparse` - RFC822 body parsing
- `rusqlite` - SQLite cache
