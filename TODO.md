# TODO

## Completed

### Gmail Labels & Folders

- [x] List all Gmail labels via IMAP (exposed as mailboxes)
- [x] Display labels in the UI
- [x] Allow filtering/viewing emails by label (`g` key)
- [x] Support special Gmail folders (Sent, Spam, Trash, Drafts, All Mail, Starred)
- [x] Display current folder in header

### Multiple Select and Bulk Actions (Search Mode)

- [x] Multi-select mode in search results (`space` to toggle, `a` to select all)
- [x] Bulk delete for selected emails
- [x] Bulk mark as read for selected emails (`m` key)
- [x] Show selection count in status bar

### AI Summarization

- [x] Integrate AI CLI tools (Claude, Codex, Gemini, Ollama)
- [x] Add summarize shortcut (`s` in read view)
- [x] Display summary in modal dialog

### Calendar Integration (macOS)

- [x] macOS EventKit integration via CGO
- [x] Calendar TUI (`maily c` / `maily calendar`)
- [x] Natural language event creation (`maily c add "tomorrow 9am meeting with Jerry"`)
- [x] AI-powered date/time parsing (prompt-based JSON extraction)
- [x] Alarm/notification support (macOS handles notifications automatically)
- [x] Prompt for reminder if not specified in natural language
- [x] Confirmation before creating event
- [x] `--debug` flag to show raw AI response
- [x] Interactive calendar selection (prompts user to pick from list)
- [x] `maily c list` to list available calendars
- [x] TUI quick-add with NLP (`a` key), falls back to interactive form when no AI CLI

### Core Features

- [x] Local email cache for fast startup
- [x] Background sync daemon
- [x] Self-update functionality
- [x] No optimistic UI - wait for server confirmation on delete
- [x] Today view (split panel: emails + events)
- [x] Extract events from email (`e` key)

### CLI Search with Pagination

- [x] `maily search -q "<query>"` - CLI search with non-interactive output
  - `-a` to specify account (required if multiple accounts)
  - Uses Gmail's native query syntax (X-GM-RAW)
  - `--format=json|table` for non-interactive output
  - `--count` for total count only
  - `--limit` and `--offset` for pagination
  - TUI mode with lazy loading (`l` to load more)

### Better Delete UX

- [x] Delete dialog with 3 options: Move to Trash (default), Permanent Delete, Cancel
- [x] Trash folder discovery (Gmail `[Gmail]/Trash`, standard IMAP `\Trash` attribute, fallbacks)
- [x] Arrow keys to select option, Enter to confirm

### Configurable AI Providers

- [x] Unified `ai_providers` config supporting both CLI tools and APIs
- [x] Each provider has: type (cli/api), name, model (required), base_url, api_key
- [x] Providers tried in order from first to last (fallback chain)
- [x] Auto-detect CLIs: claude, codex, gemini, opencode, crush, mistral, vibe, ollama
- [x] Config TUI updated to manage providers

### AI Setup Wizard

- [x] Show confirmation dialog when AI feature used but no provider found
- [x] Enter to launch config TUI, Esc to skip
- [x] Config TUI allows adding CLI or API providers

---

## Future

### AI Provider Preferences Per Task

First-time picker for AI tasks, remembered for next time.

- [ ] TUI: Show provider picker on first use of each task (summarize, extract event, create event)
- [ ] Save preference per task type to config: `ai_preferences: {summarize: "codex/o4-mini", ...}`
- [ ] Skip picker on subsequent uses, use saved preference
- [ ] CLI: Add `--ai-provider` flag to override config (for automation)
- [ ] Config TUI: Allow viewing/resetting saved preferences

### Support OAuth Login

For when the project has more traction. Unverified OAuth apps show scary warnings.

- [ ] Set up Google Cloud OAuth credentials
- [ ] Implement OAuth 2.0 flow with browser redirect
- [ ] Handle token storage and refresh
- [ ] Add `maily login gmail --oauth` option

### Notifications

- [ ] Native OS notifications on new email (daemon already syncs)
- [ ] Configurable: all emails, important only, or none
- [ ] Click notification to open maily

### Other Ideas

- [ ] Thread view (group emails by conversation)
- [ ] Attachment preview/download
- [ ] Email templates
- [ ] Vim-style navigation (j/k)
- [ ] Custom keybindings config

---

## Tauri Desktop App

### Compose

- [x] Implement compose view in Tauri frontend
- [x] Connect to Rust backend SMTP/send functionality
- [ ] Rich text editor or markdown support
- [ ] Recipient autocomplete from contacts/history
- [x] Attachment support (drag & drop, file picker)
- [x] Draft saving (auto-save to SQLite)
- [x] Reply/Reply All/Forward actions

### AI Features

- [x] Email summarization in desktop UI
- [x] AI-powered email drafting/suggestions (smart reply)
- [x] Smart reply generation (accept/decline/ask more)
- [x] Event extraction from emails
- [x] Natural language calendar event creation (calendar UI with NLP parsing)

### Configuration Management

- [x] Config editor UI in Tauri
- [x] AI providers configuration panel
  - [x] Add/edit/remove providers
  - [ ] Drag to reorder priority
  - [x] Test provider connection
  - [x] Show auto-detected CLI tools (runtime detection)
- [ ] Account settings management
- [x] General preferences (theme, language, etc.)

### Sidebar Account Display

- [x] Display max 3 accounts in sidebar
- [x] Overflow accounts hidden under "..." menu
- [x] Click overflow menu to see all accounts
- [x] Click any account to switch to it
- [x] Visual indicator for active account (ring highlight)
- [x] Account badge showing unread count
- [x] Drag to reorder accounts (persisted to config)

### Email Summary Caching

- [x] Add `email_summaries` table to SQLite schema
  - [x] Fields: email_uid, account, mailbox, summary, model_used, created_at
- [x] Check for existing summary before AI call
- [x] Display cached summary if available
- [x] Option to regenerate summary (bypass cache)
- [ ] Clear summary cache per email or bulk

### libghostty Integration

> **Status**: libghostty (libghostty-vt) is available but not yet stable (no tagged release as of Jan 2025).
> Integration is planned once the library reaches a stable release.
> Resources: https://github.com/ghostty-org/ghostty, https://mitchellh.com/writing/libghostty-is-coming

- [x] Research libghostty API and requirements
- [ ] Integrate libghostty as embedded terminal (waiting for stable release)
- [ ] Terminal view for running maily CLI commands
- [ ] Support for terminal themes/colors
- [ ] Copy/paste support
- [ ] Scrollback buffer

### AI Chat Integration

- [x] Chat sidebar in desktop app (Sheet component)
- [ ] Streaming response support (API returns full response)
- [x] Chat history persistence (localStorage via Zustand persist)
- [x] Context-aware chat (current email, folder, etc.)
- [x] Multi-turn conversations (conversation history in prompts)
- [x] Provider selection for chat (dropdown to choose provider)

### Calendar Integration (macOS - Tauri)

- [x] Rust FFI to EventKit via Objective-C
- [x] Authorization handling (request access, check status)
- [x] List all calendars with colors
- [x] List events in date range
- [x] Create events with alarms
- [x] Delete events
- [x] Calendar UI route (/calendar)
- [x] Week view with navigation
- [x] Natural language event parsing via AI
- [x] Floating calendar button on home page
- [ ] Month view
- [ ] Event editing
- [ ] Recurring events support
- [ ] Calendar color picker for new events

### Native macOS Integration (Future)

> Foundation for future native features via Rust FFI

- [x] EventKit (Calendar) - implemented
- [ ] Metal (GPU-accelerated rendering)
- [ ] Contacts.framework (address book)
- [ ] UserNotifications (rich notifications)
- [ ] NaturalLanguage.framework (on-device NLP)
- [ ] Vision.framework (OCR for attachments)
- [ ] CoreML (on-device ML)
- [ ] CoreSpotlight (system-wide search indexing)
