# Maily Desktop

A desktop productivity app for developers who want control over their email and calendar.

## Vision

Maily Desktop bridges the gap between terminal power and GUI convenience. It's not a replacement for the CLI - it's an enhancement. The terminal version keeps working. The desktop version adds visual context when you need it.

**Target audience:** Developers, DevOps/SRE, technical founders, power users who appreciate tools like Raycast, Obsidian, and fast keyboard-driven apps.

## Technical Stack

- **Framework:** Tauri (Rust + webview) - fast launch, small binary, native feel
- **Frontend:** TBD (Solid, Svelte, or React - whatever gives best performance)
- **AI:** NVIDIA APIs for cloud, optional local models for privacy
- **Auth:** OAuth 2.0 (no more app passwords)
- **Platforms:**
  - v1: macOS only (dogfood, smallest surface area)
  - v1.x: Linux (similar enough, audience wants it)
  - Maybe later: Windows (only if demand justifies the pain)

## Core Features

### Email

- Unified inbox across multiple accounts
- Conversation threading
- Rich HTML rendering with safe defaults
- Attachments (view, download, drag-drop to attach)
- Quick actions: archive, delete, snooze, label
- Keyboard-driven navigation throughout

### Calendar

- Day/week/month views
- Quick event creation with natural language ("coffee with John tomorrow 3pm")
- Event editing and deletion
- Multiple calendar support per account
- Integration with email (detect events in emails)

### Multi-Account

- Gmail (OAuth)
- Google Workspace (OAuth)
- Outlook/Microsoft 365 (OAuth) - future
- Generic IMAP/SMTP - future

## Power User Features

### Integrated Terminal

- Terminal pane that can be toggled (like VSCode)
- Run Maily CLI commands directly
- Pipe email content to shell commands
- Output from commands can reference emails/events

### Command Palette

- `Cmd+K` / `Ctrl+K` to open
- Search emails, events, contacts
- Execute actions (compose, reply, archive, etc.)
- Run custom commands/scripts
- Fuzzy matching

### Keyboard-First UX

- Full keyboard navigation (no mouse required)
- Customizable keybindings
- Single-key shortcuts for common actions (like the CLI)
- No vim requirement - sensible defaults that power users expect

### Scripting & Automation

- JavaScript API for extensions
- Hooks: on-email-receive, on-send, on-calendar-event, etc.
- Custom commands that appear in command palette
- Template system for common replies
- Rules engine (if X then Y)

Example script (Lua):
```lua
maily.on("email:receive", function(email)
  if string.find(email.from, "github.com") then
    maily.label(email, "GitHub")
    maily.archive(email)
  end
end)
```

**Why Lua?**
- Tiny (~200KB), fast (LuaJIT option)
- Proven in dev tools: Neovim, Redis, Hammerspoon, WezTerm
- Simple syntax, easy to learn
- Great Rust bindings (`mlua`)

### Split Panes

- Email list + reading pane
- Terminal + email
- Calendar + email
- Resizable, remembers layout

### Notifications

- Native OS notifications (macOS, Windows, Linux)
- New email alerts (configurable: all, important only, none)
- Calendar reminders (already works via macOS EventKit)
- Quiet hours / Do Not Disturb setting
- Sound options (on/off, custom sounds)
- Click notification → opens email/event in app

Daemon already syncs in background - just needs to trigger notifications on new mail.

## AI Features

### Compose Assistant

- Draft emails from brief prompts
- Adjust tone (formal, casual, friendly, direct)
- Expand bullet points into full emails
- Translate to other languages

### Reply Assistant

- Suggest replies based on email content
- One-click quick replies for common responses
- Context-aware (knows previous thread)

### Summarization

- TL;DR for long emails
- Thread summary for long conversations
- Daily digest summary

### Smart Features

- Priority inbox (AI ranks importance)
- Auto-categorization
- Meeting time suggestions based on calendar
- Follow-up reminders ("you said you'd send X")

### Privacy Option

- Option to run local models (Ollama, llama.cpp)
- No email content sent to cloud
- Toggle per-account or globally

## UX Principles

1. **Fast** - Launch in <500ms, instant UI responses
2. **Keyboard-first** - Everything accessible without mouse
3. **Non-blocking** - Background sync, optimistic UI where safe
4. **Minimal chrome** - Content over UI decoration
5. **Respectful** - No dark patterns, no upsells, no tracking

## Monetization (Ideas)

- Free tier: 1 account, basic AI features
- Pro tier: Multi-account, full AI, priority support
- Team tier: Shared templates, team analytics

Or: fully open source, paid cloud sync/AI features.

## Open Questions

1. What frontend framework? Solid for performance? Svelte for simplicity?
2. Local-first sync (like Obsidian) or cloud-dependent?
3. Plugin marketplace or just open extension folder?
4. Mobile app eventually? Or desktop + CLI only?
5. Notification strategy - how aggressive?

## Non-Goals

- Not trying to be Superhuman (we're not charging $30/month for polish)
- Not trying to be Thunderbird (not a kitchen sink)
- Not building for non-technical users (they have Apple Mail)
- Not building team collaboration features (not Slack)

## MVP Scope

For v0.1, ship the basics well:

1. OAuth login (Gmail only)
2. Email: list, read, compose, reply, delete
3. Calendar: view, create event
4. Command palette
5. Basic AI compose/reply (one provider)
6. Keyboard navigation
7. Single account

Everything else comes after validating people want this.
