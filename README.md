# Maily

A fast, keyboard-driven terminal email client with calendar integration.

## Features

- **Multi-account support** - Gmail, Yahoo, and custom IMAP providers
- **Fast startup** - Local caching with background sync daemon
- **Keyboard-driven interface** - Vim-inspired navigation, command palette
- **Email operations** - Compose, reply, delete, search, folder/label navigation
- **Calendar integration** - macOS EventKit with natural language event creation
- **AI summarization** - Email summaries via Claude, Codex, Gemini, Ollama, or BYOK
- **Today view** - Combined view of emails and calendar events
- **Bulk actions** - Multi-select in search results for batch operations
- **Self-update** - Built-in update mechanism

## Platforms

- **macOS** - Full features including calendar integration
- **Linux** - All features except calendar (no EventKit)

## Installation

### Homebrew (macOS/Linux)

```bash
brew install guiyumin/tap/maily
```

### From Source

```bash
git clone https://github.com/guiyumin/maily.git
cd maily
make build
./build/maily
```

### Self-Update

```bash
maily update
```

## Quick Start

1. Add your email account:

```bash
maily login gmail      # For Gmail
maily login yahoo      # For Yahoo
maily login imap       # For other IMAP providers
```

2. Start the TUI:

```bash
maily
```

The background sync daemon starts automatically when you open maily.

## Key Bindings

### List View

| Key     | Action                |
| ------- | --------------------- |
| `enter` | Open email            |
| `n`     | New email             |
| `r`     | Reply to email        |
| `R`     | Refresh from server   |
| `d`     | Delete email          |
| `s`     | Search                |
| `g`     | Switch folders/labels |
| `l`     | Load more emails      |
| `/`     | Command palette       |
| `tab`   | Switch accounts       |
| `q`     | Quit                  |

### Read View

| Key   | Action         |
| ----- | -------------- |
| `r`   | Reply          |
| `s`   | Summarize (AI) |
| `esc` | Back to list   |

### Search Results

| Key     | Action             |
| ------- | ------------------ |
| `space` | Toggle selection   |
| `a`     | Select/deselect all|
| `d`     | Delete selected    |
| `m`     | Mark as read       |
| `esc`   | Back to list       |

### Calendar View

| Key   | Action             |
| ----- | ------------------ |
| `←→`  | Previous/next day  |
| `↑↓`  | Previous/next week |
| `tab` | Cycle through events |
| `m`   | Month mode         |
| `y`   | Year mode          |
| `t`   | Jump to today      |
| `n`   | New event (NLP or form) |
| `e`   | Edit event         |
| `x/d` | Delete event       |
| `q`   | Quit               |

## Commands

```bash
# Email
maily                  # Start TUI
maily login gmail      # Add Gmail account
maily login yahoo      # Add Yahoo account
maily login imap       # Add other IMAP account
maily logout           # Remove account
maily accounts         # List accounts
maily search           # Search emails (interactive)
maily sync             # Manual full sync

# Calendar (macOS)
maily calendar         # Calendar TUI
maily c                # Short alias
maily c list           # List available calendars
maily c add "..."      # Create event with natural language

# Today View
maily today            # Combined email + calendar view
maily t                # Short alias

# Daemon
maily daemon status    # Check daemon status and logs
maily daemon stop      # Stop the daemon
maily daemon start     # Run daemon in foreground (for debugging)

# Configuration
maily config           # Interactive config TUI

# Maintenance
maily update           # Update to latest version
maily version          # Show version info
```

## Configuration

Configuration is stored in `~/.config/maily/`:

- `accounts.yml` - Email accounts and credentials
- `config.yml` - Application settings
- `cache/` - Email cache (14-day retention)
- `daemon.pid` - Background daemon PID

### Settings

```yaml
# ~/.config/maily/config.yml
max_emails: 50        # Emails to load per page
default_label: INBOX  # Default folder
theme: default        # UI theme

# AI accounts (OpenAI-compatible API)
ai_accounts:
  - name: openai
    base_url: https://api.openai.com/v1
    api_key: sk-...
    model: gpt-4o-mini
```

## Gmail Setup

1. Enable 2-Factor Authentication on your Google account
2. Generate an App Password: Google Account > Security > App Passwords
3. Use the 16-character App Password when running `maily login gmail`

## Yahoo Setup

1. Enable 2-Factor Authentication on your Yahoo account
2. Generate an App Password: Account Security > Generate app password
3. Use the App Password when running `maily login yahoo`

## AI Integration

Maily supports AI-powered features through multiple providers:

- **CLI tools**: Claude, Codex, Gemini, Ollama (auto-detected)
- **BYOK (Bring Your Own Key)**: Any OpenAI-compatible API (configure in `config.yml`)

Features:
- Email summarization (`s` key in read view)
- Natural language calendar event creation (`n` key in calendar)
- Event extraction from emails

## Architecture

- Built with Go and [Bubbletea](https://github.com/charmbracelet/bubbletea) (Elm-architecture TUI framework)
- Uses [go-imap/v2](https://github.com/emersion/go-imap) for IMAP and SMTP
- Local cache for fast startup, background daemon for sync (30-min interval)
- No optimistic UI - server operations wait for confirmation
- macOS calendar via EventKit (CGO)

## License

MIT
