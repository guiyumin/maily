# AI Chat Feature Plan

## Overview

Add an AI chat feature to Maily that enables:
1. Email summarization and intelligent Q&A
2. Natural language calendar event creation
3. Context-aware assistance within the TUI

## Use Cases

### 1. Email Summarization
- Summarize current email thread
- Summarize all unread emails
- Extract action items from emails
- Answer questions about email content

### 2. Calendar Integration (Natural Language)

**CLI:**
```bash
maily c add "tomorrow 9am talk to Jerry"
maily c add "lunch with Sarah next Friday at noon"
maily c add "team standup every Monday 10am"
```

**TUI Event Add (hybrid approach):**
When pressing `a` to add event, user can choose:
```
┌─ Add Event ──────────────────────────────────────┐
│                                                  │
│  [1] Quick add (natural language)                │
│  [2] Manual input (form fields)                  │
│                                                  │
└──────────────────────────────────────────────────┘
```

Option 1 - Natural language:
```
┌─ Quick Add ──────────────────────────────────────┐
│                                                  │
│  > tomorrow 9am meeting with boss_               │
│                                                  │
│  Press Enter to parse, Esc to cancel             │
└──────────────────────────────────────────────────┘
```

Option 2 - Form fields (current UI):
```
┌─ Add Event ──────────────────────────────────────┐
│  Title: Meeting with boss                        │
│  Date:  2024-12-24                               │
│  Time:  09:00                                    │
│  Duration: 1h                                    │
└──────────────────────────────────────────────────┘
```

Both paths end at the same confirmation screen before saving.

### 3. Email-to-Calendar Extraction
- Detect dates/times/events mentioned in emails
- Offer to add detected events to calendar
- Examples:
  - "Let's meet Thursday at 3pm" → detected, prompt to add
  - "Deadline: Dec 31st" → detected, prompt to add reminder
  - Flight confirmations, restaurant reservations, etc.

**TUI Flow:**
```
┌─ Email from Jerry ─────────────────────────────┐
│ Hey, let's grab coffee tomorrow at 10am        │
│ at Blue Bottle on Market St.                   │
└────────────────────────────────────────────────┘

 [e] Add to calendar: "Coffee with Jerry - Tomorrow 10am"
```

Press `e` to extract and add to calendar with one keystroke.

### 4. Slash Commands (TUI only)

Press `/` to open command palette with fuzzy search:

```
┌─ Commands ───────────────────────────────────────┐
│  /                                               │
│                                                  │
│  > summarize      Summarize this email           │
│    extract        Extract events to calendar     │
│    add            Add new calendar event         │
│    reply          Reply to this email            │
│    delete         Delete this email              │
│    refresh        Refresh inbox                  │
│    settings       Open settings                  │
└──────────────────────────────────────────────────┘
```

Type to filter, arrow keys to navigate, Enter to select.

**Context-aware:** Commands shown depend on current view:
- Email content view: summarize, extract, reply, delete
- Email list view: search, refresh, compose, add event
- Today view: summarize, extract, reply, delete, add event
- Calendar view: add, edit, delete event

**Context-aware shortcuts:**

The same key does different things based on context:

| Shortcut | Mail List View | Email Content View | Today View |
|----------|----------------|-------------------|------------|
| `s` | Search | Summarize (AI) | Summarize (AI) |
| `r` | Reply | Reply | Reply |
| `d` | Delete | Delete | Delete |
| `e` | - | Extract to calendar (AI) | Extract to calendar (AI) |
| `a` | - | - | Add event |

**Slash commands** (`/`) provide discoverability - same actions, searchable.

Both paths do the same thing - `/` is discoverable, shortcuts are fast.

### 5. Reply to Email

Press `r` or `/reply` to open reply compose view:

```
┌─ Reply ──────────────────────────────────────────┐
│ From: you@gmail.com                              │
│ To:   sender@example.com                         │
│ Subject: Re: Original Subject                    │
│                                                  │
│ ┌─ Body ───────────────────────────────────────┐ │
│ │                                              │ │
│ │ (cursor here)                                │ │
│ │                                              │ │
│ │ ---                                          │ │
│ │ On Dec 23, sender@example.com wrote:         │ │
│ │ > Original email content here                │ │
│ │ > quoted with > prefix                       │ │
│ └──────────────────────────────────────────────┘ │
│                                                  │
│ [Ctrl+Enter] Send   [Tab] Next field   [Esc] Cancel │
└──────────────────────────────────────────────────┘
```

**Features:**
- From: auto-filled with current account email
- To: auto-filled with original sender (Reply-To or From)
- Subject: auto-filled with "Re: " + original subject
- Body: cursor at top, original email quoted below with `>` prefix
- Quote header: "On {date}, {sender} wrote:"

**No AI needed** - this is a standard email feature.

### 6. Today View (Daily Dashboard)

Command: `maily today` or `maily t`

Split-panel view combining emails and events:

```
┌─ Today's Emails ─────────────────┬─ Events ──────────────┐
│ Meeting notes from Jerry         │ 9:00am                │
│ Q4 Budget Review                 │  Standup              │
│ Re: Project Timeline             │                       │
│ Invoice #1234                    │ 10:30am               │
│ Welcome to our newsletter        │  Meeting with boss    │
│                                  │                       │
│                                  │ 2:00pm                │
│                                  │  Client call          │
│                                  │                       │
│                                  │ 5:30pm                │
│                                  │  Gym                  │
└──────────────────────────────────┴───────────────────────┘
 [j/k] navigate  [enter] open  [a] add event  [e] edit  [d] delete
```

**Email Panel (Left):**
- Compact: title only (no date, no sender)
- Same navigation as full mail list (j/k, enter to open)
- Shows today's emails only (or unread?)

**Events Panel (Right):**
- Vertical timeline format
- Time on its own line, event title indented below
- Simple and scannable
- [a] add, [e] edit, [d] delete events

## Architecture

### CLI Commands

```bash
# Today view (daily dashboard)
maily today                          # or: maily t
                                     # Split view: emails + events

# Calendar shortcuts
maily c add "<natural language>"     # Add event via NLP
maily c list                         # List upcoming events

# Chat/AI commands
maily chat "<question>"              # One-shot question
maily chat                           # Enter interactive chat mode
```

### Components

```
internal/
├── ai/
│   ├── client.go          # AI provider abstraction (OpenAI, Anthropic, local)
│   ├── prompts.go         # System prompts for different tasks
│   ├── parser.go          # Parse NLP responses into structured data
│   └── context.go         # Build context from emails/calendar
├── calendar/
│   └── nlp.go             # Natural language date/time parsing
└── ui/
    └── components/
        ├── commandpalette.go  # Slash command palette (/)
        ├── todayview.go       # Today dashboard (emails + events split)
        ├── compactmail.go     # Compact email list (title only)
        └── eventlist.go       # Vertical event timeline
```

### AI Provider Strategy

**Target users:** CLI power users who already have AI tools installed.

**Reuse existing AI CLIs** (zero setup!):
- Claude Code: `claude -p "prompt" --output-format json`
- Codex: `codex exec "prompt" --json`
- Gemini: `gemini -p "prompt" --output-format json`
- Mistral Vibe: `vibe --prompt "prompt"`
- Ollama: `ollama run llama3.2:3b "prompt"`

Auto-detect which CLI is available, just use it for everything.

### Confirmation Flow (important!)

LLMs can hallucinate. Always show parsed result and ask for confirmation before any action.

**Example: `maily c add "lunch with bob next tuesday noon"`**
```
┌─ Confirm Event ──────────────────────────────────┐
│                                                  │
│  Title:  Lunch with Bob                          │
│  Date:   Tuesday, Dec 31, 2024                   │
│  Time:   12:00 PM                                │
│  Duration: 1 hour                                │
│                                                  │
│  [Enter] Confirm   [e] Edit   [Esc] Cancel       │
└──────────────────────────────────────────────────┘
```

**Example: `e` to extract from email**
```
┌─ Found Event in Email ───────────────────────────┐
│                                                  │
│  "Let's meet Thursday at 3pm at the office"      │
│                                                  │
│  → Title:  Meeting                               │
│  → Date:   Thursday, Dec 26, 2024                │
│  → Time:   3:00 PM                               │
│                                                  │
│  [Enter] Add to calendar   [e] Edit   [Esc] Skip │
└──────────────────────────────────────────────────┘
```

**Example: `s` to summarize email (only in email content view)**
```
┌─ Summary ────────────────────────────────────────┐
│                                                  │
│  Bob is requesting a meeting to discuss Q1       │
│  budget. He proposes Thursday 3pm. Action items: │
│  - Review attached spreadsheet                   │
│  - Prepare Q4 numbers                            │
│                                                  │
│  [Esc] Close                                     │
└──────────────────────────────────────────────────┘
```

Read-only, no confirmation needed. Only available when viewing email content.

### Natural Language Date Parsing

For `maily c add`, we need to parse natural language into structured event data:

```go
type ParsedEvent struct {
    Title              string
    StartTime          time.Time
    EndTime            time.Time  // Optional, default 1 hour
    Location           string
    AlarmMinutesBefore int        // 0 = no alarm
    AlarmSpecified     bool       // true if user mentioned reminder
}
```

**Approach: Prompt-Based JSON Extraction**

We use a prompt-based approach rather than relying on CLI-specific flags like `--json-schema`:

1. **Prompt defines the schema** - Tell AI exactly what JSON structure to return
2. **Strip markdown fences** - AI often wraps JSON in \`\`\`json...\`\`\`, so we strip it
3. **Parse and validate** - Unmarshal JSON into Go struct
4. **Create event via EventKit** - Use macOS native calendar API

**Why this approach?**

| Approach | Pros | Cons |
|----------|------|------|
| **Prompt + strip fences** | Portable across all AI CLIs, won't break if CLI changes, we control the schema | Must handle markdown wrapping |
| **CLI `--json-schema`** | Cleaner output | CLI-specific, may change, ties us to CLI internals |

The prompt-based approach is more stable because:
- We control the prompt and parsing logic
- Works identically across Claude, Codex, Gemini, Ollama
- If CLI flags change tomorrow, our code still works
- Simple 10-line function handles all edge cases

**Implementation:**
```go
// Prompt tells AI what JSON to return
prompt := `Parse this into a calendar event. Respond with ONLY JSON:
{"title":"...", "start_time":"2024-12-25T10:00:00-08:00", ...}`

response, _ := aiClient.Call(prompt)

// Strip markdown fences (AI habit of formatting code)
response = stripMarkdownCodeFences(response)

// Parse into struct
var event ParsedEvent
json.Unmarshal([]byte(response), &event)
```

**Alarm Prompt Flow:**
If user doesn't specify "remind me X minutes before", we prompt:
```
How many minutes before to remind you? (0 = no reminder):
```

### Email Context Building

For summarization, build context from:
- Current email body + headers
- Thread history (if available)
- Sender information

## Implementation Phases

### Phase 0: Core Features (No AI)
- [x] Add `maily today` / `maily t` command
- [ ] Create compact email list component (title only)
- [ ] Create vertical event list component
- [ ] Build split-panel today view
- [ ] Add event CRUD (add/edit/delete) via keyboard
- [ ] Tab or arrow keys to switch between panels
- [x] Reply feature (`r` shortcut, compose view with quoted original)
- [x] Slash command palette (`/` to open, fuzzy search, context-aware)
- [x] Context-aware `s` key (search in list view, summarize in content/today view)

### Phase 1: AI Integration (single phase for all AI features)
- [x] Auto-detect available AI CLI (claude, codex, gemini, ollama)
- [x] Implement `callAI(prompt) -> response` helper
- [x] Create confirmation dialog component (show AI result, allow edit/confirm/cancel)
- [x] `maily c add "tomorrow 9am meeting"` - NLP event creation with confirmation
- [x] Alarm prompt if user doesn't specify reminder time
- [ ] TUI quick-add with NLP (hybrid: quick add vs form)
- [ ] `e` key to extract events from email → confirm before adding
- [ ] `s` key to summarize email (in email content view and today view, read-only)
- [ ] `maily chat "question"` - one-shot Q&A

## Configuration

Add to `~/.config/maily/config.yml`:

```yaml
ai:
  provider: auto  # auto-detect, or: claude, codex, gemini, vibe, ollama
```

**Auto-detection order:** claude → codex → gemini → vibe → ollama

**Implementation:**
```go
func detectAI() string {
    for _, cli := range []string{"claude", "codex", "gemini", "vibe", "ollama"} {
        if commandExists(cli) { return cli }
    }
    return "" // no AI available
}

func callAI(prompt string) (string, error) {
    switch detectAI() {
    case "claude":
        return exec.Command("claude", "-p", prompt, "--output-format", "json").Output()
    case "codex":
        return exec.Command("codex", "exec", prompt, "--json").Output()
    case "gemini":
        return exec.Command("gemini", "-p", prompt, "--output-format", "json").Output()
    case "vibe":
        return exec.Command("vibe", "--prompt", prompt).Output()
    case "ollama":
        return exec.Command("ollama", "run", "llama3.2:3b", prompt).Output()
    }
    return "", errors.New("no AI CLI found - install claude, codex, gemini, vibe, or ollama")
}
```

## Dependencies

- macOS EventKit (native calendar integration, macOS only)
- One of: Claude Code CLI, Codex CLI, Gemini CLI, Mistral Vibe CLI, or Ollama
