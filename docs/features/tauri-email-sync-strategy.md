# Tauri Email Sync Strategy

This document describes how the Tauri desktop app synchronizes emails between IMAP servers and the local SQLite cache.

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
    account_name: String,
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

## Sync Strategy

Matches the Go TUI implementation: **max(100 emails, 14 days)**.

### Constants

```rust
const MIN_SYNC_EMAILS: usize = 100;  // Minimum emails to sync
const SYNC_DAYS: i64 = 14;           // Days to look back
const PREFETCH_BODY_COUNT: usize = 10; // Bodies to prefetch
```

### Sync Algorithm

Located in `src-tauri/src/mail.rs` → `sync_emails_with_session()`.

```
Step 1: Fetch last 100 emails by sequence number
        └─► UIDs + FLAGS + INTERNALDATE (fast, no body)

Step 2: Search for emails from last 14 days
        └─► IMAP SINCE search for UIDs

Step 3: Find missing emails (in 14-day window but not in step 1)
        └─► Fetch UIDs + FLAGS for those

Step 4: Download full emails for new UIDs
        └─► RFC822 in batches of 50

Step 5: Update flags for existing emails
        └─► Compare server flags with cached flags

Step 6: Remove stale emails from cache
        └─► Delete emails not on server (deleted on other devices)

Step 7: Prefetch body for 10 most recent
        └─► Ensures recent emails are immediately readable

Step 8: Save mailbox metadata
        └─► UIDVALIDITY + LastSync timestamp
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
    subject TEXT,
    snippet TEXT,
    body_html TEXT,
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
| Insert if missing | `save_email_metadata_to_db()` | Won't overwrite existing |
| Update body | `update_email_body_in_db()` | Updates body_html + snippet |
| Delete stale | `delete_stale_emails()` | Removes UIDs not on server |
| Get cached UIDs | `get_cached_uids()` | Returns all UIDs in cache |

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
let prefetch_uids = get_uids_without_body(conn, account, mailbox, 10);
for uid in prefetch_uids {
    // Fetch RFC822 and update body in cache
}
```

### Lazy Load (On Demand)

Other emails fetch body when user opens them:

```
User opens email → get_email() → body empty? → fetch from IMAP → cache
```

## Event Flow

```
Frontend                    Backend                     IMAP Server
   │                           │                            │
   │─── start_sync ───────────►│                            │
   │                           │─── queue_sync() ──────────►│
   │◄── sync-started ─────────│                            │
   │                           │                            │
   │                           │◄── fetch emails ──────────│
   │                           │─── update cache ──────────►│
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

## Comparison with Go TUI

| Aspect | Go TUI | Tauri Desktop |
|--------|--------|---------------|
| Process model | Separate server + client | Single process |
| Connection pool | Server-side, Unix socket | In-process, direct |
| Sync algorithm | Same | Same (100 + 14 days) |
| Background sync | 10-min poller in server | 10-min tokio timer |
| Operation queue | Server-side pending_ops | Per-account mpsc channels |
| Cache | Memory + Disk | Disk (SQLite) |
