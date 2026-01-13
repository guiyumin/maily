# Go Single Process Architecture

A long-running server model for the Go CLI, similar to tmux or emacs daemon.

**Status: IMPLEMENTED**

## Implementation Summary

The single-process server architecture is now implemented:

- **Server**: `internal/server/` - unix socket server with in-memory locking
- **Client**: `internal/client/` - TUI connection to server
- **CLI**: `maily server start|stop|status` commands
- **TUI**: Delegates operations to server (with fallback to direct IMAP)

The TUI now connects to the server for:
- Loading emails (`GetEmails`)
- Search (`Search`)
- Delete operations (`DeleteEmail`, `DeleteMulti`)
- Move to trash (`MoveToTrash`, `MoveMultiToTrash`)
- Mark as read (`MarkMultiRead`)
- Get labels (`GetLabels`)

If the server is unavailable, the TUI falls back to direct IMAP access.

---

## Old Model (Two Processes)

```
maily (TUI)     ←→     maily daemon
   ↓                        ↓
 exits              keeps running

Problems:
- File-based locking for coordination
- TUI startup loads cache from disk every time
- Two separate processes to manage
```

## Current Model (Single Process)

```
maily server (always running)
    ↑
    │ unix socket
    ↓
maily TUI (attaches/detaches)
```

### User Experience

```bash
# First launch - starts server, attaches TUI
$ maily
# Server starts in background, TUI connects
# User quits TUI (q)
# Server keeps running, syncing in background

# Next launch - just attaches to existing server
$ maily
# Detects server already running, connects to it
# Instant startup (cache already in memory)

# Explicit server control
$ maily server status    # Check if running
$ maily server stop      # Stop server
$ maily server start     # Start without attaching TUI
```

## Architecture

```
┌─────────────────────────────────────────────────┐
│            maily server (long-running)          │
│                                                 │
│  ┌───────────────────────────────────────────┐ │
│  │           Sync Manager                     │ │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐   │ │
│  │  │ Account │  │ Account │  │ Account │   │ │
│  │  │ 1 state │  │ 2 state │  │ 3 state │   │ │
│  │  └─────────┘  └─────────┘  └─────────┘   │ │
│  │       ↑            ↑            ↑        │ │
│  │       └────────────┼────────────┘        │ │
│  │              sync.RWMutex                │ │
│  └───────────────────────────────────────────┘ │
│                                                 │
│  ┌───────────────────────────────────────────┐ │
│  │     Background Poller (every 30 min)      │ │
│  └───────────────────────────────────────────┘ │
│                                                 │
│  ┌───────────────────────────────────────────┐ │
│  │     In-memory email cache                 │ │
│  │     (loaded from disk on server start)    │ │
│  └───────────────────────────────────────────┘ │
│                                                 │
│  ┌───────────────────────────────────────────┐ │
│  │     Unix socket: /tmp/maily.sock          │ │
│  │     (or ~/.config/maily/maily.sock)       │ │
│  └───────────────────────────────────────────┘ │
└─────────────────────────────────────────────────┘
         ↑                    ↑
         │ attach             │ attach
    ┌────┴────┐          ┌────┴────┐
    │  TUI 1  │          │  TUI 2  │
    │ (term1) │          │ (term2) │
    └─────────┘          └─────────┘
```

## Benefits

| Aspect | Current (Two Process) | Proposed (Single Process) |
|--------|----------------------|---------------------------|
| Locking | File-based (.sync.lock) | In-memory (sync.RWMutex) |
| TUI startup | Load cache from disk | Already in memory |
| State sharing | None | Multiple TUIs see same state |
| Coordination | PID checking | Direct mutex |
| Debugging | Check two processes | One process to inspect |

## Implementation

### Server

```go
// internal/server/server.go

type Server struct {
    accounts  *AccountManager
    cache     *MemoryCache
    poller    *Poller
    listener  net.Listener
    clients   map[*Client]bool
    mu        sync.RWMutex
}

type AccountManager struct {
    states map[string]*AccountState
    mu     sync.RWMutex  // Per-operation lock, not per-account file lock
}

type AccountState struct {
    Email      string
    Mailboxes  map[string]*MailboxState
    Syncing    bool  // In-memory flag, no file lock needed
    LastSync   time.Time
}

func NewServer(sockPath string) (*Server, error) {
    listener, err := net.Listen("unix", sockPath)
    if err != nil {
        return nil, err
    }

    s := &Server{
        accounts: NewAccountManager(),
        cache:    NewMemoryCache(),
        listener: listener,
        clients:  make(map[*Client]bool),
    }

    // Load cache from disk into memory
    s.cache.LoadFromDisk()

    // Start background poller
    s.poller = NewPoller(s.accounts, s.cache)
    go s.poller.Run()

    return s, nil
}

func (s *Server) Run() {
    for {
        conn, err := s.listener.Accept()
        if err != nil {
            continue
        }
        client := NewClient(conn, s)
        go client.Handle()
    }
}
```

### Client (TUI)

```go
// internal/client/client.go

type TUIClient struct {
    conn   net.Conn
    reader *bufio.Reader
    writer *bufio.Writer
}

func Connect(sockPath string) (*TUIClient, error) {
    conn, err := net.Dial("unix", sockPath)
    if err != nil {
        return nil, err
    }
    return &TUIClient{
        conn:   conn,
        reader: bufio.NewReader(conn),
        writer: bufio.NewWriter(conn),
    }, nil
}

func (c *TUIClient) SendKeypress(key tea.KeyMsg) error {
    // Encode and send keypress to server
}

func (c *TUIClient) ReceiveState() (*UIState, error) {
    // Receive UI state update from server
}
```

### Main Entry Point

```go
// main.go

func main() {
    sockPath := filepath.Join(os.Getenv("HOME"), ".config", "maily", "maily.sock")

    if serverRunning(sockPath) {
        // Server exists, just attach TUI
        runTUIClient(sockPath)
    } else {
        // Start server in background, then attach
        startServerBackground(sockPath)
        time.Sleep(100 * time.Millisecond)  // Wait for socket
        runTUIClient(sockPath)
    }
}

func serverRunning(sockPath string) bool {
    conn, err := net.Dial("unix", sockPath)
    if err != nil {
        // Clean up stale socket file
        os.Remove(sockPath)
        return false
    }
    conn.Close()
    return true
}

func startServerBackground(sockPath string) {
    // Fork or use exec to start server in background
    cmd := exec.Command(os.Args[0], "server", "start")
    cmd.Start()
}
```

### Protocol

Simple JSON-based protocol over unix socket:

```go
// Client → Server
type ClientMessage struct {
    Type    string      `json:"type"`    // "keypress", "resize", "command"
    Payload interface{} `json:"payload"`
}

// Server → Client
type ServerMessage struct {
    Type    string      `json:"type"`    // "state", "notification", "error"
    Payload interface{} `json:"payload"`
}

// Full UI state (sent on connect and after changes)
type UIState struct {
    CurrentAccount  string           `json:"current_account"`
    CurrentMailbox  string           `json:"current_mailbox"`
    Emails          []EmailSummary   `json:"emails"`
    SelectedIndex   int              `json:"selected_index"`
    View            string           `json:"view"`  // "list", "read", "compose"
    // ... etc
}
```

## In-Memory Locking

No more file-based locks for Go ↔ Go coordination:

```go
type AccountManager struct {
    states map[string]*AccountState
    mu     sync.RWMutex
}

func (am *AccountManager) StartSync(account string) (bool, error) {
    am.mu.Lock()
    defer am.mu.Unlock()

    state := am.states[account]
    if state.Syncing {
        return false, nil  // Already syncing
    }
    state.Syncing = true
    return true, nil
}

func (am *AccountManager) EndSync(account string) {
    am.mu.Lock()
    defer am.mu.Unlock()
    am.states[account].Syncing = false
}
```

## Coexistence with Tauri

When both Go server and Tauri desktop might access the cache:

```
┌──────────────┐          ┌──────────────┐
│  Go Server   │          │ Tauri App    │
│              │          │              │
│ In-memory    │          │ In-memory    │
│ locking      │          │ locking      │
└──────┬───────┘          └──────┬───────┘
       │                         │
       │   File lock for         │
       │   cross-process         │
       └─────────┬───────────────┘
                 ↓
         ~/.config/maily/cache/
```

### Option A: File Lock for Cross-Process

Keep file lock, but only check it when starting sync:

```go
func (am *AccountManager) StartSync(account string) (bool, error) {
    // 1. Check in-memory lock (Go ↔ Go)
    am.mu.Lock()
    state := am.states[account]
    if state.Syncing {
        am.mu.Unlock()
        return false, nil
    }

    // 2. Check file lock (Go ↔ Tauri)
    if externalProcessHasLock(account) {
        am.mu.Unlock()
        return false, nil
    }

    // 3. Acquire both
    state.Syncing = true
    acquireFileLock(account)
    am.mu.Unlock()

    return true, nil
}
```

### Option B: Ownership Model

Define clear ownership - only one app writes at a time:

```yaml
# ~/.config/maily/config.yml
cache_owner: go  # or "tauri"
```

- Owner app: full read/write access
- Other app: read-only, or must request access

### Option C: Separate Caches

Each app has its own cache, sync independently:

```
~/.config/maily/cache-go/      # Go server's cache
~/.config/maily/cache-tauri/   # Tauri's cache
```

Wasteful of disk and bandwidth, but zero coordination needed.

**Recommended: Option A** - keeps shared cache, minimal coordination overhead.

## Migration Path

1. **Phase 1**: Add server mode alongside current daemon
   - `maily server start/stop/status`
   - TUI can connect to server or run standalone

2. **Phase 2**: Make server the default
   - `maily` auto-starts server if not running
   - Deprecate `maily daemon`

3. **Phase 3**: Remove old daemon code
   - Server is the only background mode
   - File locks only for Tauri coexistence

## Files Changed

```
internal/
├── server/
│   ├── server.go      # Main server loop
│   ├── client.go      # Client connection handler
│   ├── protocol.go    # Message types
│   └── memory.go      # In-memory cache
├── client/
│   └── tui.go         # TUI that connects to server
├── cli/
│   ├── root.go        # Updated to connect or start server
│   └── server.go      # New: server subcommand
└── cache/
    └── cache.go       # Add LoadToMemory/SyncToDisk methods
```

## Open Questions

1. **Rendering**: Does server send full UI state, or does client render locally?
   - Full state: simpler protocol, more bandwidth
   - Local render: client needs all Bubbletea logic

2. **Graceful shutdown**: How to handle `maily server stop` with connected clients?
   - Notify clients, wait for disconnect?
   - Force disconnect after timeout?

3. **Auto-start on login**: Should server start automatically?
   - launchd plist (macOS)
   - systemd unit (Linux)

4. **Resource limits**: Memory cap for in-memory cache?
   - LRU eviction?
   - Spill to disk after N emails?
