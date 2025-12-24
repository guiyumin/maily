# Maily

A fast, keyboard-driven terminal email client.

## Features

- Multi-account support (Gmail, Yahoo, and other IMAP providers)
- Fast startup with local caching
- Keyboard-driven interface
- Compose, reply, delete emails
- Search across emails
- Folder/label navigation
- Background sync daemon

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

3. (Optional) Start background sync daemon:
```bash
maily daemon start
```

## Key Bindings

### List View

| Key | Action |
|-----|--------|
| `enter` | Open email |
| `c` | Compose new email |
| `r` | Reply to email |
| `R` | Refresh from server |
| `d` | Delete email |
| `s` | Search |
| `g` | Switch folders/labels |
| `l` | Load more emails |
| `/` | Command palette |
| `tab` | Switch accounts |
| `q` | Quit |

### Read View

| Key | Action |
|-----|--------|
| `r` | Reply |
| `s` | Summarize (AI) |
| `esc` | Back to list |

## Commands

```bash
maily                  # Start TUI
maily login gmail      # Add Gmail account
maily login yahoo      # Add Yahoo account
maily login imap       # Add other IMAP account
maily logout           # Remove account
maily accounts         # List accounts
maily daemon start     # Start background sync
maily daemon stop      # Stop background sync
maily sync             # Manual full sync
maily update           # Update to latest version
```

## Configuration

Accounts are stored in `~/.config/maily/accounts.yml`

Email cache is stored in `~/.config/maily/cache/`

## Gmail Setup

1. Enable 2-Factor Authentication on your Google account
2. Generate an App Password: Google Account > Security > App Passwords
3. Use the 16-character App Password when running `maily login gmail`

## Architecture

- Built with Go and [Bubbletea](https://github.com/charmbracelet/bubbletea) (Elm-architecture TUI framework)
- Uses [go-imap/v2](https://github.com/emersion/go-imap) for IMAP
- Local cache for fast startup, background daemon for sync
- No optimistic UI - server operations wait for confirmation

## License

MIT
