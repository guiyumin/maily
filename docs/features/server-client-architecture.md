# Server-Client Architecture

This document describes the server-client architecture introduced in v0.8.0, which separates the long-running IMAP connection management from the TUI client.

## Overview

```
┌─────────────────┐     Unix Socket     ┌─────────────────────────────┐
│   TUI Client    │ ◄─────────────────► │         Server              │
│  (internal/ui)  │   JSON protocol     │   (internal/server)         │
└─────────────────┘                     │                             │
                                        │  ┌─────────────────────┐   │
                                        │  │   StateManager      │   │
                                        │  │  - Account states   │   │
                                        │  │  - IMAP pool        │   │
                                        │  │  - Memory cache     │   │
                                        │  │  - Disk cache       │   │
                                        │  └─────────────────────┘   │
                                        │            │                │
                                        │            ▼                │
                                        │  ┌─────────────────────┐   │
                                        │  │    IMAP Server      │   │
                                        │  │   (Gmail, etc.)     │   │
                                        │  └─────────────────────┘   │
                                        │                             │
                                        │  ┌─────────────────────┐   │
                                        │  │  Background Poller  │   │
                                        │  │  - 10-min sync      │   │
                                        │  │  - 10-sec ops queue │   │
                                        │  └─────────────────────┘   │
                                        └─────────────────────────────┘
```

**Key Design Principle:** The TUI client has **zero direct IMAP connections**. All IMAP operations go through the server, which maintains a pooled connection per account (4 accounts = 4 total IMAP connections).

## Components

### Server (`internal/server/`)

#### `server.go` - Main Server Process

The server is a long-running process that:
- Listens on Unix socket at `~/.config/maily/maily.sock`
- Writes PID to `~/.config/maily/server.pid`
- Manages multiple TUI client connections
- Runs background sync poller

```go
type Server struct {
    sockPath string
    listener net.Listener
    state    *StateManager
    clients  map[*Client]bool  // connected TUI clients
    done     chan struct{}
}
```

**Key functions:**
- `New()` - Creates server, loads accounts, initializes cache
- `Run()` - Starts accept loop and background poller
- `handleClient()` - Handles a single client connection
- `handleRequest()` - Routes requests to appropriate handlers
- `broadcastEvent()` - Sends events to all connected clients
- `backgroundPoller()` - Runs sync and ops processing on timers

#### `state.go` - State Manager

Manages all account states and provides synchronized access to IMAP connections and cache.

```go
type StateManager struct {
    accounts map[string]*AccountState  // keyed by email
    store    *auth.AccountStore
    cache    *cache.Cache              // disk cache (SQLite)
    memory   *MemoryCache              // in-memory cache
}

type AccountState struct {
    Account    *auth.Account
    Syncing    bool
    LastSync   time.Time
    LastError  error
    imapClient *mail.IMAPClient  // pooled connection
}
```

**Key functions:**
- `withIMAPClient()` - Execute operation with pooled IMAP connection
- `GetEmails()` - Load from disk cache, fallback to memory
- `GetEmailWithBody()` - Load email, fetch body from IMAP if missing
- `Sync()` - Full sync: last 100 emails + 14-day window
- `QueueOp()` / `QueueOps()` - Queue operations for background processing
- `ProcessPendingOps()` - Process queued delete/move operations

#### `protocol.go` - Wire Protocol

JSON-based request/response protocol over Unix socket.

**Request Types:**

| Type | Category | Description |
|------|----------|-------------|
| `hello` | Handshake | Version handshake |
| `ping` | Handshake | Health check |
| `get_accounts` | Read | List all accounts |
| `get_emails` | Read | Get emails for account/mailbox |
| `get_email` | Read | Get single email with body |
| `get_labels` | Read | Get mailbox list |
| `get_sync_status` | Read | Get sync status |
| `sync` | Async | Trigger background sync |
| `mark_read` / `mark_unread` | Synchronous | Update read status |
| `mark_multi_read` | Synchronous | Mark multiple as read |
| `delete_email` / `delete_multi` | Synchronous | Immediate delete |
| `move_to_trash` / `move_multi_trash` | Synchronous | Immediate move to trash |
| `queue_delete` / `queue_delete_multi` | Queued | Queued delete (fast UI) |
| `queue_move_trash` / `queue_move_multi_trash` | Queued | Queued move to trash (fast UI) |
| `search` | Synchronous | Search emails via IMAP |
| `quick_refresh` | Synchronous | Manual metadata refresh |
| `save_draft` | Synchronous | Save email to Drafts folder |
| `download_attachment` | Synchronous | Download attachment to disk |
| `shutdown` | Control | Stop server |

**Response Types:**
| Type | Description |
|------|-------------|
| `ok` | Success (no data) |
| `error` | Error with message |
| `hello` | Version handshake response |
| `emails` | Email list |
| `email` | Single email |
| `labels` | Mailbox list |
| `status` | Sync status |
| `accounts` | Account list |
| `pong` | Ping response |

**Event Types (Server → Client Push):**
| Type | Description |
|------|-------------|
| `sync_started` | Sync began for account |
| `sync_completed` | Sync finished |
| `sync_error` | Sync failed |
| `new_emails` | New emails arrived |
| `email_updated` | Email flags changed |

#### `memory.go` - In-Memory Cache

Fast in-memory email cache for quick access. Used as secondary cache after disk.

### Client (`internal/client/`)

The TUI client connects to the server for **all** IMAP operations.

```go
type Client struct {
    conn    net.Conn
    reader  *bufio.Reader
    encoder *json.Encoder
    reqID   uint64
    pending map[string]chan server.Response  // request ID → response channel
    events  chan server.Event                // push events from server
}
```

**Key functions:**
- `Connect()` - Connect to server, perform version handshake
- `GetEmails()`, `GetEmail()`, `GetLabels()` - Read operations
- `MarkRead()`, `MarkUnread()`, `MarkMultiRead()` - Flag operations
- `DeleteEmail()`, `DeleteMulti()` - Immediate delete
- `QueueDeleteEmail()`, `QueueDeleteMulti()` - Queued delete
- `MoveToTrash()`, `MoveMultiToTrash()` - Immediate move
- `QueueMoveToTrash()`, `QueueMoveMultiToTrash()` - Queued move
- `Search()` - Search emails
- `QuickRefresh()` - Manual metadata refresh
- `SaveDraft()` - Save draft to Drafts folder
- `DownloadAttachment()` - Download attachment, returns file path
- `Sync()` - Trigger background sync
- `Events()` - Channel for push events

## Operations Flow

### Operation Categories

All operations go through the server. They are categorized by their execution model:

#### 1. Synchronous Operations (Real-Time)

These operations block until the IMAP server responds. The TUI waits for completion.

| Operation | Request Type | Description |
|-----------|--------------|-------------|
| Mark Read/Unread | `mark_read`, `mark_unread` | Update flags, wait for IMAP |
| Immediate Delete | `delete_email`, `delete_multi` | Delete, wait for IMAP |
| Immediate Move | `move_to_trash`, `move_multi_trash` | Move, wait for IMAP |
| Search | `search` | Execute IMAP search, return results |
| Quick Refresh | `quick_refresh` | Fetch metadata, update cache |
| Save Draft | `save_draft` | Append to Drafts folder |
| Download Attachment | `download_attachment` | Fetch attachment, save to disk |

**Flow:**
```
TUI → Server: "save this draft"
Server → IMAP: execute immediately
IMAP → Server: "done"
Server → TUI: "done" or "error"
```

#### 2. Queued Operations (Eventual Consistency)

These operations return immediately after updating cache. IMAP operation happens in background.

| Operation | Request Type | Description |
|-----------|--------------|-------------|
| Queued Delete | `queue_delete`, `queue_delete_multi` | Remove from cache, queue IMAP op |
| Queued Move | `queue_move_trash`, `queue_move_multi_trash` | Remove from cache, queue IMAP op |

**Flow:**
```
TUI → Server: "queue delete"
Server: removes from cache immediately
Server → TUI: "ok, queued" (immediate response)
Server → IMAP: (later, in background poller)
```

#### 3. Async Operations (Background)

These trigger background work and return immediately.

| Operation | Request Type | Description |
|-----------|--------------|-------------|
| Sync | `sync` | Trigger background sync |

**Flow:**
```
TUI → Server: "sync"
Server → TUI: "ok, syncing"
Server: broadcasts sync_started event
Server → IMAP: (fetch emails in background)
Server: broadcasts sync_completed or sync_error event
```

### Detailed Operation Descriptions

1. **Loading Emails** (`get_emails`)
   - Server loads from disk cache (SQLite)
   - Falls back to memory cache if disk unavailable
   - Returns cached emails to client

2. **Reading Email Body** (`get_email`)
   - Server checks disk cache for body
   - If missing, fetches from IMAP using pooled connection
   - Updates both disk and memory cache
   - Returns email with body to client

3. **Quick Refresh** (`quick_refresh`)
   - Server fetches last 100 emails + 14-day window metadata
   - Inserts missing emails into cache
   - Updates cache freshness timestamp
   - Returns refreshed email list

4. **Save Draft** (`save_draft`)
   - Server appends message to Drafts folder via IMAP
   - Waits for IMAP confirmation
   - Returns success/failure

5. **Download Attachment** (`download_attachment`)
   - Server fetches attachment content via IMAP
   - Decodes (base64/quoted-printable)
   - Saves to `~/Downloads/maily/` (with dedup)
   - Returns file path to TUI

6. **Search** (`search`)
   - Server executes IMAP search (X-GM-RAW for Gmail, TEXT for others)
   - Returns results directly (not cached)

## Cache Architecture

### Two-Level Cache

```
┌─────────────────┐
│  Memory Cache   │  ← Fast access, per-session
│  (MemoryCache)  │
└────────┬────────┘
         │ fallback
         ▼
┌─────────────────┐
│   Disk Cache    │  ← Persistent, SQLite
│  (~/.config/    │
│   maily/maily.db)│
└─────────────────┘
```

**Read Order:**
1. Try disk cache first (source of truth)
2. If found, also store in memory cache
3. If disk empty, try memory cache
4. If both empty, return nil (will trigger IMAP fetch)

### Cache Freshness

- Server tracks `LastSync` timestamp per account/mailbox
- Cache considered "fresh" if synced within 10 minutes
- Fresh cache skips initial sync on startup
- `IsCacheFresh()` checks cache metadata

### Pending Operations Queue

Queued operations allow fast UI response while ensuring eventual consistency:

```sql
CREATE TABLE pending_ops (
    id INTEGER PRIMARY KEY,
    account TEXT,
    mailbox TEXT,
    operation TEXT,  -- 'delete' or 'move_trash'
    uid INTEGER,
    created_at TIMESTAMP,
    attempts INTEGER DEFAULT 0,
    last_error TEXT
);
```

- Operations removed from cache immediately
- Background poller processes every 10 seconds
- Retries on connection errors
- Logs success/failure to `ops_log` table

## Connection Management

### Server-Side IMAP Pool

Each account has **one** pooled IMAP connection:

```go
func (sm *StateManager) withIMAPClient(email string, fn func(*mail.IMAPClient) error) error {
    state.imapMu.Lock()
    defer state.imapMu.Unlock()

    client, err := sm.ensureIMAPClientLocked(state)  // create if nil
    if err != nil {
        return err
    }

    err = fn(client)
    if isConnectionError(err) {
        state.imapClient.Close()
        state.imapClient = nil  // will reconnect on next call
    }
    return err
}
```

Benefits:
- Reuses connections across operations
- Auto-reconnects on connection errors
- Serializes access with mutex (prevents concurrent IMAP operations)

### Client-Side Connection

TUI maintains **only** a server connection (no direct IMAP):

```go
serverClient, _ := client.Connect()  // in NewApp()
```

Falls back to disk cache if server unavailable.

**Total connections for 4 accounts:** 4 (all on server side)

## Background Sync

### Sync Interval

- Server syncs all accounts every 10 minutes
- TUI auto-refreshes every 5 minutes (if idle)
- Manual refresh with `R` key (uses `quick_refresh`)

### Sync Algorithm

```
1. Fetch last 100 emails by sequence number (metadata only)
2. Fetch UIDs from last 14 days
3. Find UIDs in 14-day window not in the 100
4. Fetch those missing emails (metadata only)
5. Store in cache (insert if missing)
6. Update cache metadata (LastSync timestamp)
```

This ensures:
- Always have recent 100 emails
- Never miss emails from last 14 days
- Efficient: only fetches what's missing

## Version Compatibility

Client and server versions must match exactly:

```go
func (c *Client) hello() error {
    resp, err := c.request(server.Request{
        Type:    server.ReqHello,
        Version: version.Version,
    }, 5*time.Second)
    // Returns ErrVersionMismatch if versions differ
}
```

If versions mismatch, client shows error:
```
version mismatch: client=0.8.1, server=0.8.0 - please run 'maily server stop' and restart
```

## File Locations

| File | Purpose |
|------|---------|
| `~/.config/maily/maily.sock` | Unix socket |
| `~/.config/maily/server.pid` | Server PID + version |
| `~/.config/maily/maily.db` | SQLite cache |
| `~/.config/maily/accounts.yml` | Account credentials |
| `~/.config/maily/config.yml` | App settings |

## Starting/Stopping Server

```bash
# Start server (foreground)
maily server start

# Start server (background)
maily server start &

# Check status
maily server status

# Stop server
maily server stop
```

The server is automatically started when running `maily` if not already running.

## Design Rationale

### Why Server-Client Architecture?

1. **Connection Persistence** - IMAP connections are expensive to establish. A long-running server can maintain persistent connections.

2. **Background Sync** - Server can sync emails in background without blocking the UI.

3. **Shared State** - Multiple TUI instances can share the same server and cache.

4. **Resource Efficiency** - Single IMAP connection per account. With 4 accounts, only 4 total IMAP connections exist (all on server).

5. **Queued Operations** - Delete/move operations can be queued for reliability without blocking UI.

### Why All IMAP Through Server?

The TUI has **zero direct IMAP connections**. All operations go through the server because:

1. **Centralized Connection Pool** - Server manages all IMAP connections. No duplicate connections.

2. **Consistent State** - Server is the single source of truth for email state.

3. **Simplified TUI** - TUI only needs to handle UI logic, not IMAP protocol.

4. **Synchronous Operations Work** - Operations like draft saving and attachment download are synchronous (wait for IMAP response) but still go through the server for connection reuse.

### Synchronous vs Queued Operations

| Operation Type | When to Use | Example |
|----------------|-------------|---------|
| Synchronous | User needs immediate feedback | Save draft, download attachment |
| Queued | User doesn't need to wait | Delete email (remove from UI immediately) |

**SMTP is separate:** `sendReply()` creates its own SMTP connection because SMTP is a different protocol from IMAP. This is the only "exception" but it's not an IMAP operation.
