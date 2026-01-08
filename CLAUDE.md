# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build Commands

```bash
make build      # Build to build/maily
make clean      # Remove build directory
```

## Architecture

Maily is a terminal email client built with Go and Bubbletea (Elm-architecture TUI framework).

### Core Structure

- `main.go` - CLI entry point using Cobra
- `internal/cli/` - CLI commands: login, logout, accounts, daemon, sync, update
- `internal/auth/` - Account management, credentials stored in `~/.config/maily/accounts.yml`
- `internal/gmail/` - IMAP client (fetching, read/unread, delete) and SMTP client (send, reply)
- `internal/ui/` - Bubbletea TUI application
- `internal/cache/` - Local email cache for fast startup
- `internal/updater/` - Self-update functionality

### UI Architecture (Bubbletea)

The UI follows Elm architecture with `Init()`, `Update()`, `View()` methods:

- `internal/ui/app.go` - Main App model, handles state transitions, keyboard/mouse events
- `internal/ui/components/` - Reusable components (maillist, command palette, label picker)
- `internal/ui/components/views.go` - View rendering functions
- `internal/ui/components/styles.go` - Lipgloss styles (purple primary theme)
- `internal/ui/commands.go` - Async commands (delete, refresh, search, etc.)

### Key Patterns

- **Multi-account support**: Accounts stored in `AccountStore`, Tab key switches between them
- **Local cache**: Emails cached to disk for fast startup, daemon syncs in background
- **No optimistic UI**: Server operations wait for confirmation before updating UI
- **Gmail App Passwords**: Uses IMAP/SMTP with App Passwords (not OAuth) for simplicity

### Key Bindings (List View)

- `enter` - Open email
- `n` - New email
- `r` - Reply to email
- `R` - Refresh from server
- `d` - Delete email (with confirmation)
- `s` - Search
- `g` - Switch folders/labels
- `l` - Load more emails
- `/` - Command palette
- `tab` - Switch accounts
- `q` - Quit

### Key Bindings (Calendar View)

- `←→` - Previous/next day
- `↑↓` - Previous/next week
- `tab` - Cycle through events
- `m` - Month mode
- `y` - Year mode
- `t` - Jump to today
- `n` - New event (NLP if AI CLI available, otherwise interactive form)
- `e` - Edit event
- `x/d` - Delete event
- `q` - Quit

### Delete Flow

No optimistic UI - wait for server confirmation:

1. `d` → confirmation dialog
2. `y` → spinner, send to server, wait for response
3. Success → remove from UI and cache
4. Error → show error, email stays in UI

### Authentication Flow

1. User runs `maily login gmail`
2. Prompts for email and App Password (password input hidden)
3. Verifies credentials by connecting to IMAP before saving
4. Stores in `~/.config/maily/accounts.yml`

### IMAP Notes

- Uses `go-imap/v2` library
- `Peek: true` in fetch options prevents marking emails as read when loading
- Import `github.com/emersion/go-message/charset` for proper character encoding

### Cache Design

See `docs/cache.md` for detailed cache architecture. Key points:

- Server is single source of truth
- Local cache is just a mirror for fast startup
- Daemon syncs every 30 minutes
- Manual refresh with `R` fetches from server

## Clean Code Reviewer

**Before generating or modifying code**, you MUST:

1. Check if `.cleancoderules/` exists.
2. If it exists, read and apply those rules.
3. If it does not exist, proceed with Level 2/1 rules only.

**After generating or modifying code**, you MUST:

1. Use the `review_code` MCP tool if it is configured in this environment.
2. Otherwise run `ccr review <file>` for each modified file.
3. Fix any violations before presenting the code.

### Rule Priority

- Level 3: Team rules in `.cleancoderules/team/` (highest)
- Level 2: Community rules in `.cleancoderules/community/`
- Level 1: Base rules (general principles)

If rules conflict, always follow the higher-level rule.
