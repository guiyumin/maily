# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

```bash
make build           # Build to build/maily
make clean           # Remove build directory
make lint            # Run staticcheck
make version patch   # Bump patch version (0.0.X)
make version minor   # Bump minor version (0.X.0)
make version major   # Bump major version (X.0.0)
make push            # Push to origin with tags
```

## Architecture

Maily is a terminal email client built with Go and Bubbletea (Elm-architecture TUI framework).

### Directory Structure

```
maily/
├── main.go                    # CLI entry point
├── internal/
│   ├── cli/                   # Cobra CLI commands
│   │   ├── root.go           # Root command, launches TUI
│   │   ├── login.go          # Account login flow
│   │   ├── logout.go         # Account removal
│   │   ├── accounts.go       # List accounts
│   │   ├── calendar.go       # Calendar TUI launcher
│   │   ├── today.go          # Today view launcher
│   │   ├── server_cmd.go     # Server start/stop/status commands
│   │   ├── sync_cmd.go       # Manual sync command
│   │   ├── config.go         # Config TUI launcher
│   │   ├── config_tui.go     # Config TUI implementation
│   │   ├── search.go         # CLI search
│   │   ├── update.go         # Self-update
│   │   └── version.go        # Version display
│   ├── ui/                    # Bubbletea TUI application
│   │   ├── app.go            # Main App model (1500+ lines)
│   │   ├── calendar.go       # Calendar TUI
│   │   ├── today.go          # Combined email + calendar view
│   │   ├── compose.go        # Email composition
│   │   ├── search.go         # Search results view
│   │   ├── commands.go       # Async tea.Cmd handlers
│   │   ├── calendar_*.go     # Calendar views and forms
│   │   └── components/       # Reusable UI components
│   │       ├── styles.go     # Lipgloss styles (purple theme)
│   │       ├── views.go      # View rendering functions
│   │       ├── maillist.go   # Email list component
│   │       ├── commandpalette.go
│   │       ├── labelpicker.go
│   │       ├── datepicker.go
│   │       ├── timepicker.go
│   │       └── filepicker.go
│   ├── mail/                  # Email protocol implementation
│   │   ├── imap.go           # IMAP client (go-imap/v2)
│   │   ├── smtp.go           # SMTP client
│   │   ├── search.go         # IMAP search (X-GM-RAW support)
│   │   └── provider.go       # Gmail/Yahoo folder constants
│   ├── auth/                  # Account management
│   │   └── credentials.go    # ~/.config/maily/accounts.yml
│   ├── cache/                 # Local email storage
│   │   └── cache.go          # Per-account disk cache
│   ├── sync/                  # Email synchronization
│   │   └── sync.go           # Sync logic (used by server)
│   ├── server/                # Long-running server process
│   │   ├── server.go         # Unix socket server, background poller
│   │   ├── state.go          # Account state, sync logic
│   │   └── protocol.go       # Client-server protocol
│   ├── client/                # TUI client for server
│   │   └── client.go         # Unix socket client
│   ├── calendar/              # Calendar integration
│   │   ├── calendar.go       # Abstract interface
│   │   ├── eventkit_darwin.go # macOS EventKit (CGO)
│   │   └── stub_other.go     # Stub for non-macOS
│   ├── ai/                    # AI integration
│   │   ├── client.go         # Multi-provider AI client
│   │   └── prompts.go        # AI prompt templates
│   ├── proc/                  # Process management
│   │   └── proc.go           # PID-based locking
│   ├── updater/               # Self-update
│   │   └── updater.go        # GitHub release updates
│   └── version/               # Version info
│       └── version.go        # Injected via LDFLAGS
├── config/                    # Configuration
│   └── config.go             # YAML config management
└── docs/                      # Documentation
```

### UI Architecture (Bubbletea)

The UI follows Elm architecture with `Init()`, `Update()`, `View()` methods:

- **app.go** - Main App model, handles state transitions, keyboard/mouse events
- **components/** - Reusable components (maillist, command palette, label picker)
- **views.go** - View rendering functions
- **styles.go** - Lipgloss styles (purple primary theme #7C3AED)
- **commands.go** - Async commands (delete, refresh, search, etc.)

### Key Patterns

- **Multi-account support**: Accounts in `AccountStore`, Tab key switches
- **Local cache**: Emails cached to disk for fast startup, server syncs in background
- **No optimistic UI**: Server operations wait for confirmation before updating UI
- **Gmail App Passwords**: Uses IMAP/SMTP with App Passwords (not OAuth)
- **Message-passing**: Commands return `tea.Cmd` for async operations

### Key Bindings

See [docs/keybindings.md](docs/keybindings.md) for the full list of key bindings.

### Delete Flow

No optimistic UI - wait for server confirmation:

1. `d` → confirmation dialog (Move to Trash / Permanent Delete / Cancel)
2. Select option → spinner, send to server, wait for response
3. Success → remove from UI and cache
4. Error → show error, email stays in UI

### Authentication Flow

1. User runs `maily login gmail` / `maily login yahoo` / `maily login qq`
2. Prompts for email and App Password (password input hidden)
3. Verifies credentials by connecting to IMAP before saving
4. Stores in `~/.config/maily/accounts.yml`

### IMAP Notes

- Uses `go-imap/v2` library
- `Peek: true` in fetch options prevents marking emails as read when loading
- Import `github.com/emersion/go-message/charset` for proper character encoding
- Gmail X-GM-RAW for native search syntax

### Search Feature

See `docs/features/search.md` for detailed search architecture. Key points:

- Two modes: Interactive TUI and non-interactive CLI (JSON/table output)
- `-a` flag required when multiple accounts configured
- Gmail X-GM-RAW for native search syntax, TEXT search for other providers
- Lazy loading: UIDs fetched first (fast), then emails loaded on demand
- TUI: Press `l` to load 50 more emails, status shows loaded/total count
- CLI flags: `--count`, `--format`, `--limit`, `--offset` trigger non-interactive mode

### Cache Design

See `docs/cache.md` for detailed cache architecture. Key points:

- Server is single source of truth
- Local cache is just a mirror for fast startup
- Server syncs every 30 minutes
- Manual refresh with `R` fetches from server
- 14-day retention by INTERNALDATE
- UIDVALIDITY change detection (clears cache if UIDs reassigned)
- Per-account locking prevents concurrent syncs

### AI Integration

Multi-provider support with fallback:

1. **CLI tools** (auto-detected): Claude, Codex, Gemini, Ollama
2. **BYOK (Bring Your Own Key)**: Any OpenAI-compatible API in `~/.config/maily/config.yml`

Used for:

- Email summarization (`s` key in read view)
- Natural language calendar event creation
- Event extraction from emails

### Calendar Integration (macOS)

- EventKit integration via CGO (`internal/calendar/eventkit_darwin.go`)
- Abstract interface for cross-platform (`internal/calendar/calendar.go`)
- Stub implementation for non-macOS (`internal/calendar/stub_other.go`)
- NLP event creation with AI-powered date/time parsing

### Configuration Storage

- Accounts: `~/.config/maily/accounts.yml` (0600 permissions)
- Config: `~/.config/maily/config.yml`
- Cache: `~/.config/maily/maily.db` (SQLite database)
- Server PID: `~/.config/maily/server.pid`
- Server socket: `~/.config/maily/maily.sock`

# Rules

- **MUST** SAY "Yes, Yumin" in the beginning of any reply
- **MUST** NEVER do git add and commit
- **MUST** USE bun, instead of npm, for tauri app
