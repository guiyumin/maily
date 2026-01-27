# Email UTF-8 Parsing Bug Fix

## Problem

Emails with invalid UTF-8 characters in text fields were silently dropped when loading from the SQLite cache. This caused:

1. **Missing emails** - The total count showed correctly (e.g., 59), but fewer emails were displayed (e.g., 50)
2. **Error when viewing emails** - "Conversion error from type Text at index: N, invalid utf-8 sequence"

## Root Cause

The Rust `rusqlite` library's `row.get::<_, String>()` method assumes valid UTF-8 encoding. When an email contains invalid UTF-8 bytes (common in emails from non-UTF-8 encodings like GB2312, Latin-1, etc.), the parsing fails.

The code used `filter_map(|r| r.ok())` which silently dropped rows that failed to parse:

```rust
// Before: silently drops emails with invalid UTF-8
let emails: Vec<EmailSummary> = stmt.query_map(params, |row| {
    Ok(EmailSummary {
        from: row.get(3)?,      // FAILS if invalid UTF-8
        subject: row.get(5)?,   // FAILS if invalid UTF-8
        snippet: row.get(7)?,   // FAILS if invalid UTF-8
        // ...
    })
})?.filter_map(|r| r.ok()).collect();  // Bad rows silently dropped
```

## Solution

Created a helper function that uses lossy UTF-8 conversion - invalid bytes are replaced with the Unicode replacement character (U+FFFD, displayed as `�`):

```rust
/// Helper to get text field with lossy UTF-8 conversion
fn get_text_lossy(row: &rusqlite::Row, idx: usize) -> rusqlite::Result<String> {
    match row.get_ref(idx)? {
        rusqlite::types::ValueRef::Text(bytes) => Ok(String::from_utf8_lossy(bytes).into_owned()),
        rusqlite::types::ValueRef::Blob(bytes) => Ok(String::from_utf8_lossy(bytes).into_owned()),
        rusqlite::types::ValueRef::Null => Ok(String::new()),
        _ => Ok(String::new()),
    }
}
```

Applied to all text fields in email reading functions:

```rust
// After: handles invalid UTF-8 gracefully
Ok(EmailSummary {
    from: get_text_lossy(row, 3)?,
    subject: get_text_lossy(row, 5)?,
    snippet: get_text_lossy(row, 7)?,
    // ...
})
```

## Files Changed

- `tauri/src-tauri/src/mail.rs` - Added `get_text_lossy()` helper and updated all email reading functions:
  - `list_emails_paginated()`
  - `list_unread_emails()`
  - `get_emails()`
  - `get_email()`
  - `search_emails()`

## Affected Fields

All text fields that could contain user-generated or email content:
- `message_id`
- `from`
- `to`
- `cc`
- `reply_to`
- `subject`
- `date` (string fallback)
- `snippet`
- `body_html`

## Testing

After the fix:
- All emails load correctly (59/59 instead of 50/59)
- Emails with invalid UTF-8 display with `�` replacement characters
- No more "Conversion error" when viewing emails

## Date

2026-01-27
