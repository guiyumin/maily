# Email Tagging System Plan

Replace email snippets with custom tags + AI auto-tagging in the Tauri app.

## Summary
- **Remove snippet** from email list, replace with tag badges
- **Local SQLite tags** (no IMAP sync, no foreign keys)
- **AI auto-tags** during sync for prefetched emails (10 most recent)
- **Tag dialog** in email reader: manual input, select existing, or AI generate
- **Delete tags** anytime from email content view

---

## Phase 1: Database Schema

**File: `tauri/src-tauri/src/mail.rs`**

Add to SCHEMA constant (no foreign keys for simplicity):
```sql
CREATE TABLE IF NOT EXISTS tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    color TEXT NOT NULL DEFAULT '#7C3AED',
    created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS email_tags (
    account TEXT NOT NULL,
    mailbox TEXT NOT NULL,
    email_uid INTEGER NOT NULL,
    tag_id INTEGER NOT NULL,
    auto_generated INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    PRIMARY KEY (account, mailbox, email_uid, tag_id)
);

CREATE INDEX IF NOT EXISTS idx_email_tags_email ON email_tags(account, mailbox, email_uid);
CREATE INDEX IF NOT EXISTS idx_email_tags_tag ON email_tags(tag_id);
```

### Cleanup (No Foreign Keys)

Since we don't use foreign keys, manually delete email_tags when:

1. **Email deleted** - In `delete_email_from_cache()`:
   ```rust
   DELETE FROM email_tags WHERE account = ? AND mailbox = ? AND email_uid = ?
   ```

2. **Stale emails removed during sync** - In `delete_stale_emails()`:
   ```rust
   // For each deleted UID, also delete from email_tags
   DELETE FROM email_tags WHERE account = ? AND mailbox = ? AND email_uid = ?
   ```

3. **Tag deleted** - In `delete_tag()`:
   ```rust
   DELETE FROM email_tags WHERE tag_id = ?
   DELETE FROM tags WHERE id = ?
   ```

---

## Phase 2: Rust Backend

**File: `tauri/src-tauri/src/mail.rs`**

Add structs:
- `Tag { id, name, color, created_at }`
- `EmailTag { tag_id, tag_name, tag_color, auto_generated }`

Add functions (no update - just create/delete):
- `get_all_tags()` - List all tags
- `create_tag(name, color)` - Create new tag (returns existing if name matches)
- `delete_tag(tag_id)` - Delete tag AND all email_tags with that tag_id
- `get_email_tags(account, mailbox, uid)` - Get tags for one email
- `add_tag_to_email(account, mailbox, uid, tag_id, auto_generated)` - Add tag to email
- `remove_tag_from_email(account, mailbox, uid, tag_id)` - Remove tag from email
- `get_emails_tags_batch(account, mailbox, uids)` - Batch query for list view
- `delete_email_tags(account, mailbox, uid)` - Delete all tags for an email (called when email deleted)
- `search_emails_by_tags(account, mailbox, tag_ids)` - Get email UIDs that have any of the given tags

**Note:** No update operations. To change a tag, delete and recreate.

**Modify existing functions:**
- `delete_email_from_cache()` - Also call `delete_email_tags()`
- `delete_stale_emails()` - Also delete email_tags for removed UIDs

**File: `tauri/src-tauri/src/lib.rs`**

Register Tauri commands:
- `list_tags`, `create_tag`, `delete_tag`
- `get_email_tags`, `add_email_tag`, `remove_email_tag`
- `get_batch_email_tags`
- `search_emails_by_tags` - Filter emails by tag IDs
- `auto_tag_email`

---

## Phase 3: AI Auto-Tagging

**File: `tauri/src-tauri/src/ai.rs`**

Add `auto_tag_email(from, subject, body_text, existing_tags)`:
- Sends email content to AI with available tags
- AI returns up to 3 suggested tags (existing or new)
- Returns JSON array of tag names

### When AI Tagging Runs

1. **During sync** - For prefetched emails (10 most recent with body):
   - After body prefetch completes in `sync_emails_with_session()`
   - Check if email has no tags → call AI → save tags
   - Runs in background, no user interaction needed

2. **On demand** - User clicks "Generate Tags" button in tag dialog:
   - User triggers AI tagging manually
   - Useful for older emails not prefetched during sync

---

## Phase 4: TypeScript Types & API

**Create: `tauri/src/types/tags.ts`**
```typescript
export interface Tag { id: number; name: string; color: string; }
export interface EmailTag { tag_id: number; tag_name: string; tag_color: string; auto_generated: boolean; }
```

**Create: `tauri/src/lib/tags.ts`**
- `listTags()`, `createTag()`, `deleteTag()`
- `getEmailTags()`, `addEmailTag()`, `removeEmailTag()`
- `getBatchEmailTags()`, `autoTagEmail()`

---

## Phase 5: UI Components

**Create: `tauri/src/components/tags/`**
- `TagBadge.tsx` - Single colored tag badge with optional remove button
- `TagList.tsx` - Inline list of badges (for email list, max 3)
- `TagDialog.tsx` - Dialog for managing tags on an email

### TagDialog Component

Button in email content opens dialog with:
```
┌─────────────────────────────────────────┐
│  Manage Tags                        [X] │
├─────────────────────────────────────────┤
│                                         │
│  Current tags:                          │
│  [Newsletter ×] [Finance ×] [Travel ×]  │
│                                         │
│  ─────────────────────────────────────  │
│                                         │
│  Add tag:                               │
│  [_____________________] [Add]          │
│                                         │
│  Or select existing:                    │
│  [Work] [Personal] [Important] [...]    │
│                                         │
│  ─────────────────────────────────────  │
│                                         │
│  [🤖 Generate Tags with AI]             │
│                                         │
└─────────────────────────────────────────┘
```

Features:
- Display current tags with delete (×) button
- **Select existing tags** - Show all available tags, click to add to email
- **Create new tag** - Text input to create and add a new tag
- "Generate Tags with AI" button calls AI and suggests tags
- User can accept/reject AI suggestions

Adding a tag workflow:
1. Click existing tag badge → immediately adds to email
2. Or type new tag name → creates tag + adds to email
3. Or click AI generate → suggests tags → user selects which to add

**Tag colors palette:**
```typescript
const TAG_COLORS = ["#7C3AED", "#EF4444", "#F59E0B", "#10B981", "#3B82F6", "#EC4899", "#6366F1", "#14B8A6"];
```

---

## Phase 6: Search by Tag

Two ways to filter emails by tag:

### 1. Tag Filter Dropdown

Add filter dropdown next to search bar in `EmailList.tsx`:
```
[Search...________] [Filter by tag ▼]
                         ┌──────────────┐
                         │ ☐ Newsletter │
                         │ ☐ Finance    │
                         │ ☐ Travel     │
                         │ ──────────── │
                         │ Clear filter │
                         └──────────────┘
```
- Multi-select: filter by multiple tags (OR logic)
- Shows only tags that exist in current mailbox
- "Clear filter" resets to show all emails

### 2. Search Syntax

Support `tag:tagname` in search bar:
```
tag:finance              → emails with "finance" tag
tag:newsletter           → emails with "newsletter" tag
tag:finance tag:travel   → emails with either tag (OR)
from:john tag:work       → combine with other search terms
```

**Backend:** Add `search_emails_by_tags(account, mailbox, tag_ids)` function

**Frontend:** Parse search input for `tag:` prefix, extract tag names, resolve to IDs

---

## Phase 7: Email List Display

**File: `tauri/src/components/home/EmailList.tsx`**

1. Add `tags?: EmailTag[]` to Email interface
2. Replace snippet display (lines 210-212):
```tsx
// Before: snippet text
// After: <TagList tags={email.tags} /> or fallback to snippet if no tags
```

**File: `tauri/src/components/home/Home.tsx`**

After fetching emails, batch-fetch tags:
```typescript
const tagsMap = await getBatchEmailTags(account, mailbox, uids);
// Merge tags into emails
```

---

## Phase 7: Email Reader Integration

**File: `tauri/src/components/home/EmailReader.tsx`**

1. Add tags state and fetch on email change
2. Display current tags as badges (read-only display)
3. Add "Manage Tags" button that opens `<TagDialog>`
4. Tags can be added/removed/AI-generated via the dialog

```tsx
// In email header area
<div className="flex items-center gap-2">
  <TagList tags={tags} />
  <Button variant="ghost" size="sm" onClick={() => setTagDialogOpen(true)}>
    <Tag className="h-4 w-4" />
  </Button>
</div>

<TagDialog
  open={tagDialogOpen}
  onOpenChange={setTagDialogOpen}
  account={account}
  mailbox={mailbox}
  uid={email.uid}
  tags={tags}
  onTagsChange={setTags}
  emailContext={{ from: email.from, subject: email.subject, bodyText }}
/>
```

---

## Files to Modify
1. `tauri/src-tauri/src/mail.rs` - Schema, structs, DB functions, search by tag
2. `tauri/src-tauri/src/lib.rs` - Tauri commands
3. `tauri/src-tauri/src/ai.rs` - Auto-tag prompt
4. `tauri/src/components/home/EmailList.tsx` - Replace snippet with tags, add tag filter dropdown
5. `tauri/src/components/home/EmailReader.tsx` - Tag dialog button + display
6. `tauri/src/components/home/Home.tsx` - Batch tag fetching, tag filter state, search parsing

## Files to Create
1. `tauri/src/types/tags.ts`
2. `tauri/src/lib/tags.ts`
3. `tauri/src/components/tags/TagBadge.tsx`
4. `tauri/src/components/tags/TagList.tsx`
5. `tauri/src/components/tags/TagDialog.tsx` - Dialog with manual input + AI generate

---

## Verification

1. **Build backend**: `cd tauri && bun run tauri build --debug`
2. **Test tag CRUD**: Create/delete tags, verify in DB
3. **Test email tagging**: Add/remove tags on email, verify display
4. **Test batch fetch**: Verify email list shows tags correctly
5. **Test AI auto-tag**: Open email with body, verify tags suggested
6. **Test inline create**: Create new tag from tag picker
