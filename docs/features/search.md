# Search Feature

## Overview

Maily provides powerful email search with two modes:
- **Interactive TUI** - Full-screen search with selection, actions, and lazy loading
- **Non-interactive CLI** - JSON/table output for scripting and automation

## Usage

### Interactive TUI Search

```bash
maily search -q "from:temu"
maily search -a me@gmail.com -q "has:attachment is:unread"
```

### Non-interactive CLI Search

```bash
# Count only (fast - no email fetch)
maily search -q "from:temu" --count

# JSON output with pagination
maily search -q "from:temu" --format=json --limit=50
maily search -q "from:temu" --format=json --limit=50 --offset=50

# Table output
maily search -q "is:unread" --format=table
```

## CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-q, --query` | (required) | Search query |
| `-a, --account` | auto | Account email (required if multiple accounts) |
| `--count` | false | Only return total count, don't fetch emails |
| `--format` | "" | Output format: `json` or `table` (triggers non-interactive mode) |
| `--limit` | 100 | Max results to return |
| `--offset` | 0 | Skip first N results for pagination |

## Gmail Search Syntax

For Gmail accounts, full Gmail search syntax is supported via X-GM-RAW:

```
from:sender@example.com    Emails from a sender
to:recipient@example.com   Emails to a recipient
subject:hello              Emails with subject containing 'hello'
has:attachment             Emails with attachments
is:unread                  Unread emails
is:starred                 Starred emails
in:inbox                   Emails in inbox
in:trash                   Emails in trash
older_than:30d             Emails older than 30 days
newer_than:7d              Emails newer than 7 days
category:promotions        Promotional emails
category:social            Social emails
larger:5M                  Emails larger than 5MB
label:important            Emails with label
```

### Examples

```bash
# Find promotional emails older than 30 days
maily search -q "category:promotions older_than:30d"

# Unread emails with attachments
maily search -q "has:attachment is:unread"

# Emails from a specific sender in the last week
maily search -q "from:amazon newer_than:7d"
```

## Other Providers

For non-Gmail providers (Yahoo, etc.), standard IMAP TEXT search is used:
- Simple keyword search in email body and headers
- No advanced syntax support

## Architecture

### Search Flow

```
┌─────────────────────────────────────────────────────────────┐
│ Phase 1: UID Search (Fast)                                  │
│                                                             │
│   Gmail:  Raw IMAP → UID SEARCH X-GM-RAW "query"           │
│   Other:  go-imap  → client.UIDSearch(TextCriteria)        │
│                                                             │
│   Returns: []UID (can be 10,000+)                          │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│ Phase 2: Lazy Loading (On-demand)                           │
│                                                             │
│   - Only fetch email details for visible range              │
│   - Batch size: ~50 emails                                  │
│   - Triggered by: initial load, scrolling                   │
│                                                             │
│   go-imap: client.Fetch(uidSet, fetchOptions)              │
│   Returns: []Email with headers, body, attachments         │
└─────────────────────────────────────────────────────────────┘
```

### Gmail X-GM-RAW Implementation

go-imap v2 doesn't support Gmail's X-GM-RAW extension, so we bypass it:

```go
// internal/mail/search.go
func GmailSearch(creds, mailbox, query) ([]UID, error) {
    // 1. Open direct TLS connection to imap.gmail.com:993
    // 2. Send raw IMAP commands: LOGIN, SELECT, UID SEARCH X-GM-RAW
    // 3. Parse response to extract UIDs
    // 4. Return UIDs for fetching via go-imap
}
```

See `docs/bugfix/gmail-search-not-working.md` for details on why this approach was necessary.

### Data Structures

```go
// TUI Search App
type SearchApp struct {
    uids   []imap.UID           // All matching UIDs from search
    emails map[int]mail.Email   // Lazily loaded emails by index
    // ...
}

// CLI Search Response
type SearchResponse struct {
    Total   int            `json:"total"`
    Offset  int            `json:"offset"`
    Limit   int            `json:"limit"`
    Results []SearchResult `json:"results"`
}
```

## TUI Features

### Key Bindings

| Key | Action |
|-----|--------|
| `j/k` or `↑/↓` | Navigate list |
| `Enter` | Open email |
| `Space` | Toggle selection |
| `a` | Select/deselect all |
| `l` | Load 50 more emails |
| `d` | Delete selected |
| `r` | Mark selected as read |
| `Esc` | Back / Cancel |
| `q` | Quit |

### Confirmation Dialogs

Actions require confirmation with button-style UI:
- Navigate with `← →` or `h/l`
- Confirm with `Enter`
- Cancel with `Esc`

### Visual Indicators

- `●` Blue dot = Unread
- `○` Gray dot = Read
- `📎` Amber = Has attachments
- `[✓]` Green = Selected
- `[ ]` Gray = Not selected

## Performance

### Lazy Loading

For large result sets (1000+ emails):

1. **Initial search**: Only fetches UIDs (fast, ~100ms for 10,000 results)
2. **First render**: Loads first 50 emails with details
3. **Manual loading**: Press `l` to load 50 more emails
4. **Status bar**: Shows "50/1000 emails" to indicate loaded vs total count

### Memory Efficiency

- UIDs stored as slice (8 bytes each)
- Emails stored in map, only for loaded indices
- ~50-100 emails in memory at a time for large result sets

## Files

- `internal/cli/search.go` - CLI command and non-interactive mode
- `internal/ui/search.go` - TUI search application
- `internal/mail/search.go` - IMAP search (Gmail X-GM-RAW + standard)
- `internal/mail/imap.go` - FetchByUIDs for lazy loading
