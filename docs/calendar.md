# Calendar Integration Design

## Goal

Full calendar TUI invoked by `maily calendar` or `maily c`

## UI Layout

```
┌─ December 2024 ──────────────────────────────────────────────┐
│                                                              │
│  Sun     Mon     Tue     Wed     Thu     Fri     Sat        │
│   1       2       3       4       5       6       7         │
│                          •                                   │
│   8       9      10      11      12      13      14         │
│           •              ••                                  │
│  15      16      17      18      19      20     [21]        │
│                                                 •••         │
│  22      23      24      25      26      27      28         │
│   •                                                          │
│  29      30      31                                          │
│                                                              │
├──────────────────────────────────────────────────────────────┤
│                                                              │
│  Sat, Dec 21                                                 │
│                                                              │
│   9:00 AM   Team Standup                          [Work]    │
│  12:00 PM   Lunch with Bob                        [Home]    │
│   3:00 PM   1:1 with Manager                      [Work]    │
│                                                              │
├──────────────────────────────────────────────────────────────┤
│  ←/→ day  ↑/↓ week  a add  e edit  d delete  t today  q quit│
└──────────────────────────────────────────────────────────────┘
```

## Features

### Navigation
- `←` `→` - previous/next day
- `↑` `↓` - previous/next week
- `h` `l` - previous/next month (vim style)
- `t` - jump to today
- `q` or `Esc` - quit

### CRUD
- `a` - add event (opens form)
- `e` - edit selected event
- `d` - delete selected event
- `Enter` - view event details

### Add/Edit Form

```
┌─ New Event ──────────────────────────────────────┐
│                                                  │
│  Title:    [Team Meeting____________]            │
│  Date:     [2024-12-22__]                        │
│  Start:    [10:00 AM]     End: [11:00 AM]        │
│  Calendar: [▼ Work_____]                         │
│  Location: [Conference Room B_______]            │
│                                                  │
│            [Cancel]  [Save]                      │
└──────────────────────────────────────────────────┘
```

## CLI Alias

```go
// Both invoke the calendar TUI
maily calendar
maily c
```

## Implementation

### Files
- `internal/ui/calendar.go` - Main calendar TUI model
- `internal/ui/calendar_form.go` - Add/edit form component

### Dependencies
- Uses existing `internal/calendar/` EventKit integration
- Follows same Bubbletea patterns as email TUI
