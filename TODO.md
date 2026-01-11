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

### Better Delete UX

- [x] Delete dialog with 3 options: Move to Trash (default), Permanent Delete, Cancel
- [x] Trash folder discovery (Gmail `[Gmail]/Trash`, standard IMAP `\Trash` attribute, fallbacks)
- [x] Arrow keys to select option, Enter to confirm

---

## Future

### Configurable AI CLI

- [ ] Allow user to choose AI CLI provider (codex, gemini, claude)
- [ ] Add config option: `ai_cli: codex | gemini | claude`
- [ ] Claude Haiku is slow - default to faster option if available

### CLI Bulk Processing Commands

Non-interactive CLI commands for scripting/automation.

- [ ] `maily search --from=<account> --query="<query>"` - CLI search with actions
  - Uses Gmail's native query syntax (X-GM-RAW)
  - Output results as JSON or table
  - Pipe to other commands for batch operations

### Support OAuth Login

For when the project has more traction. Unverified OAuth apps show scary warnings.

- [ ] Set up Google Cloud OAuth credentials
- [ ] Implement OAuth 2.0 flow with browser redirect
- [ ] Handle token storage and refresh
- [ ] Add `maily login gmail --oauth` option

### Calendar Features (Future)

- [ ] Extract events from email (`e` key)
- [x] Today view (split panel: emails + events)

### Other Ideas

- [ ] Thread view (group emails by conversation)
- [ ] Attachment preview/download
- [ ] Email templates
- [ ] Vim-style navigation (j/k)
- [ ] Custom keybindings config
