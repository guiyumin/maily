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

- `main.go` - CLI entry point, handles commands: `login`, `logout`, `accounts`, `help`
- `internal/auth/` - Account management, credentials stored in `~/.config/maily/accounts.yml`
- `internal/gmail/` - IMAP client (fetching, read/unread, delete) and SMTP client (send, reply)
- `internal/ui/` - Bubbletea TUI application
- `config/` - App configuration (unused currently)

### UI Architecture (Bubbletea)

The UI follows Elm architecture with `Init()`, `Update()`, `View()` methods:

- `internal/ui/app.go` - Main App model, handles state transitions, keyboard/mouse events, account switching
- `internal/ui/components/maillist.go` - Email list component with scrolling and selection
- `internal/ui/styles.go` - Lipgloss styles (purple primary theme)

### Key Patterns

- **Multi-account support**: Accounts stored in `AccountStore`, Tab key switches between them
- **Email caching**: `emailCache` and `imapCache` maps preserve loaded emails when switching accounts
- **Gmail App Passwords**: Uses IMAP/SMTP with App Passwords (not OAuth) for simplicity

### Authentication Flow

1. User runs `maily login gmail`
2. Prompts for email and App Password (password input hidden, strips all unicode whitespace)
3. Verifies credentials by connecting to IMAP before saving
4. Stores in `~/.config/maily/accounts.yml`

### IMAP Notes

- Uses `go-imap/v2` library
- `Peek: true` in fetch options prevents marking emails as read when loading
- Import `github.com/emersion/go-message/charset` for proper character encoding
