# Apple Notes Integration

Full CRUD and search integration with macOS Notes.app from Maily.

## Overview

This feature allows users to manage Apple Notes directly from Maily:
- **Create**: Save emails to Notes (full content or AI summary)
- **Read**: Browse and view notes within Maily
- **Update**: Edit existing notes
- **Delete**: Remove notes
- **Search**: Find notes by content

Follows the existing pattern used for Calendar (EventKit) integration.

## Architecture

```
internal/notes/
├── notes.go           # Abstract interface
├── notes_darwin.go    # macOS implementation (ScriptingBridge via CGO)
├── notes_darwin.m     # Objective-C implementation
├── notes_darwin.h     # Objective-C header
└── stub_other.go      # Stub for non-macOS
```

### Data Types

```go
// internal/notes/notes.go
package notes

import "time"

// Note represents an Apple Note
type Note struct {
    ID           string    // ScriptingBridge note ID (for update/delete)
    Title        string
    Body         string    // HTML or plain text
    Folder       string    // Folder name
    CreatedAt    time.Time
    ModifiedAt   time.Time
}

// Folder represents a Notes folder
type Folder struct {
    ID    string
    Name  string
    Count int  // Number of notes in folder
}

// SearchResult contains note with match context
type SearchResult struct {
    Note    Note
    Snippet string  // Matched text snippet
}
```

### Interface

```go
type NotesClient interface {
    // Availability
    Available() bool

    // CRUD - Notes
    Create(note Note) (string, error)           // Returns note ID
    Get(noteID string) (*Note, error)           // Get single note
    List(folder string, limit int) ([]Note, error)  // List notes in folder
    Update(noteID string, note Note) error      // Update existing note
    Delete(noteID string) error                 // Delete note

    // Folders
    ListFolders() ([]Folder, error)
    CreateFolder(name string) error
    DeleteFolder(name string) error

    // Search
    Search(query string, folder string) ([]SearchResult, error)
}

func New() NotesClient
```

## macOS Implementation (ScriptingBridge)

Uses ScriptingBridge framework via CGO for native, high-performance access to Notes.app. No process spawning - direct Objective-C API calls.

### Header File

```objc
// internal/notes/notes_darwin.h
#ifndef NOTES_DARWIN_H
#define NOTES_DARWIN_H

#import <Foundation/Foundation.h>

// Note data structure for C/Go interop
typedef struct {
    const char *id;
    const char *title;
    const char *body;
    const char *folder;
    double createdAt;   // Unix timestamp
    double modifiedAt;  // Unix timestamp
} CNote;

// Folder data structure
typedef struct {
    const char *id;
    const char *name;
    int count;
} CFolder;

// Search result
typedef struct {
    CNote note;
    const char *snippet;
} CSearchResult;

// Result arrays with count
typedef struct {
    CNote *notes;
    int count;
} CNoteArray;

typedef struct {
    CFolder *folders;
    int count;
} CFolderArray;

typedef struct {
    CSearchResult *results;
    int count;
} CSearchResultArray;

// API functions
int NotesAvailable(void);
const char* NotesCreate(const char *title, const char *body, const char *folder);
CNote* NotesGet(const char *noteID);
CNoteArray NotesListInFolder(const char *folder, int limit);
int NotesUpdate(const char *noteID, const char *title, const char *body);
int NotesDelete(const char *noteID);
CFolderArray NotesListFolders(void);
int NotesCreateFolder(const char *name);
int NotesDeleteFolder(const char *name);
CSearchResultArray NotesSearch(const char *query, const char *folder);

// Memory cleanup
void FreeNote(CNote *note);
void FreeNoteArray(CNoteArray arr);
void FreeFolderArray(CFolderArray arr);
void FreeSearchResultArray(CSearchResultArray arr);

#endif
```

### Objective-C Implementation

```objc
// internal/notes/notes_darwin.m
#import "notes_darwin.h"
#import <Foundation/Foundation.h>
#import <ScriptingBridge/ScriptingBridge.h>

// Notes.app ScriptingBridge interface (generated from Notes.sdef)
@interface NotesApplication : SBApplication
- (SBElementArray *) folders;
- (SBElementArray *) notes;
@end

@interface NotesFolder : SBObject
@property (copy) NSString *name;
@property (copy, readonly) NSString *id;
- (SBElementArray *) notes;
@end

@interface NotesNote : SBObject
@property (copy) NSString *name;
@property (copy) NSString *body;
@property (copy, readonly) NSString *id;
@property (copy, readonly) NSDate *creationDate;
@property (copy, readonly) NSDate *modificationDate;
@property (copy, readonly) NotesFolder *container;
@end

// Helper to get Notes app instance
static NotesApplication* getNotesApp(void) {
    return (NotesApplication *)[SBApplication applicationWithBundleIdentifier:@"com.apple.Notes"];
}

// Helper to copy NSString to C string (caller must free)
static const char* toCString(NSString *str) {
    if (!str) return NULL;
    return strdup([str UTF8String]);
}

int NotesAvailable(void) {
    NotesApplication *app = getNotesApp();
    return app != nil ? 1 : 0;
}

const char* NotesCreate(const char *title, const char *body, const char *folder) {
    @autoreleasepool {
        NotesApplication *app = getNotesApp();
        if (!app) return NULL;

        NSString *folderName = folder ? [NSString stringWithUTF8String:folder] : @"Maily";
        NSString *noteTitle = [NSString stringWithUTF8String:title];
        NSString *noteBody = [NSString stringWithUTF8String:body];

        // Find or create folder
        NotesFolder *targetFolder = nil;
        for (NotesFolder *f in [app folders]) {
            if ([f.name isEqualToString:folderName]) {
                targetFolder = f;
                break;
            }
        }

        if (!targetFolder) {
            // Create folder
            NotesFolder *newFolder = [[[app classForScriptingClass:@"folder"] alloc] init];
            newFolder.name = folderName;
            [[app folders] addObject:newFolder];
            targetFolder = newFolder;
        }

        // Create note
        Class noteClass = [app classForScriptingClass:@"note"];
        NotesNote *newNote = [[noteClass alloc] init];
        newNote.name = noteTitle;
        newNote.body = noteBody;
        [[targetFolder notes] addObject:newNote];

        return toCString(newNote.id);
    }
}

CNote* NotesGet(const char *noteID) {
    @autoreleasepool {
        NotesApplication *app = getNotesApp();
        if (!app) return NULL;

        NSString *targetID = [NSString stringWithUTF8String:noteID];

        for (NotesNote *note in [app notes]) {
            if ([note.id isEqualToString:targetID]) {
                CNote *result = malloc(sizeof(CNote));
                result->id = toCString(note.id);
                result->title = toCString(note.name);
                result->body = toCString(note.body);
                result->folder = toCString(note.container.name);
                result->createdAt = [note.creationDate timeIntervalSince1970];
                result->modifiedAt = [note.modificationDate timeIntervalSince1970];
                return result;
            }
        }
        return NULL;
    }
}

CNoteArray NotesListInFolder(const char *folder, int limit) {
    @autoreleasepool {
        NotesApplication *app = getNotesApp();
        CNoteArray result = {NULL, 0};
        if (!app) return result;

        NSString *folderName = [NSString stringWithUTF8String:folder];
        NotesFolder *targetFolder = nil;

        for (NotesFolder *f in [app folders]) {
            if ([f.name isEqualToString:folderName]) {
                targetFolder = f;
                break;
            }
        }

        if (!targetFolder) return result;

        NSArray *notes = [[targetFolder notes] get];
        int count = (int)[notes count];
        if (limit > 0 && limit < count) count = limit;

        result.notes = malloc(sizeof(CNote) * count);
        result.count = count;

        for (int i = 0; i < count; i++) {
            NotesNote *note = notes[i];
            result.notes[i].id = toCString(note.id);
            result.notes[i].title = toCString(note.name);
            result.notes[i].body = NULL;  // Skip body for list performance
            result.notes[i].folder = toCString(folderName);
            result.notes[i].createdAt = [note.creationDate timeIntervalSince1970];
            result.notes[i].modifiedAt = [note.modificationDate timeIntervalSince1970];
        }

        return result;
    }
}

int NotesUpdate(const char *noteID, const char *title, const char *body) {
    @autoreleasepool {
        NotesApplication *app = getNotesApp();
        if (!app) return -1;

        NSString *targetID = [NSString stringWithUTF8String:noteID];

        for (NotesNote *note in [app notes]) {
            if ([note.id isEqualToString:targetID]) {
                if (title) note.name = [NSString stringWithUTF8String:title];
                if (body) note.body = [NSString stringWithUTF8String:body];
                return 0;
            }
        }
        return -1;  // Not found
    }
}

int NotesDelete(const char *noteID) {
    @autoreleasepool {
        NotesApplication *app = getNotesApp();
        if (!app) return -1;

        NSString *targetID = [NSString stringWithUTF8String:noteID];

        for (NotesNote *note in [app notes]) {
            if ([note.id isEqualToString:targetID]) {
                [note delete];
                return 0;
            }
        }
        return -1;
    }
}

CFolderArray NotesListFolders(void) {
    @autoreleasepool {
        NotesApplication *app = getNotesApp();
        CFolderArray result = {NULL, 0};
        if (!app) return result;

        NSArray *folders = [[app folders] get];
        result.count = (int)[folders count];
        result.folders = malloc(sizeof(CFolder) * result.count);

        for (int i = 0; i < result.count; i++) {
            NotesFolder *f = folders[i];
            result.folders[i].id = toCString(f.id);
            result.folders[i].name = toCString(f.name);
            result.folders[i].count = (int)[[[f notes] get] count];
        }

        return result;
    }
}

int NotesCreateFolder(const char *name) {
    @autoreleasepool {
        NotesApplication *app = getNotesApp();
        if (!app) return -1;

        NotesFolder *newFolder = [[[app classForScriptingClass:@"folder"] alloc] init];
        newFolder.name = [NSString stringWithUTF8String:name];
        [[app folders] addObject:newFolder];
        return 0;
    }
}

int NotesDeleteFolder(const char *name) {
    @autoreleasepool {
        NotesApplication *app = getNotesApp();
        if (!app) return -1;

        NSString *folderName = [NSString stringWithUTF8String:name];
        for (NotesFolder *f in [app folders]) {
            if ([f.name isEqualToString:folderName]) {
                [f delete];
                return 0;
            }
        }
        return -1;
    }
}

CSearchResultArray NotesSearch(const char *query, const char *folder) {
    @autoreleasepool {
        NotesApplication *app = getNotesApp();
        CSearchResultArray result = {NULL, 0};
        if (!app) return result;

        NSString *queryStr = [NSString stringWithUTF8String:query];
        NSString *folderName = folder ? [NSString stringWithUTF8String:folder] : nil;

        NSMutableArray *matches = [NSMutableArray array];

        // Get notes from specific folder or all notes
        NSArray *notesToSearch;
        if (folderName) {
            for (NotesFolder *f in [app folders]) {
                if ([f.name isEqualToString:folderName]) {
                    notesToSearch = [[f notes] get];
                    break;
                }
            }
        } else {
            notesToSearch = [[app notes] get];
        }

        // Search in title and body
        for (NotesNote *note in notesToSearch) {
            NSString *title = note.name ?: @"";
            NSString *body = note.body ?: @"";

            if ([title localizedCaseInsensitiveContainsString:queryStr] ||
                [body localizedCaseInsensitiveContainsString:queryStr]) {
                [matches addObject:note];
            }
        }

        result.count = (int)[matches count];
        result.results = malloc(sizeof(CSearchResult) * result.count);

        for (int i = 0; i < result.count; i++) {
            NotesNote *note = matches[i];
            result.results[i].note.id = toCString(note.id);
            result.results[i].note.title = toCString(note.name);
            result.results[i].note.folder = toCString(note.container.name);
            result.results[i].note.createdAt = [note.creationDate timeIntervalSince1970];
            result.results[i].note.modifiedAt = [note.modificationDate timeIntervalSince1970];

            // Create snippet from body
            NSString *body = note.body ?: @"";
            NSRange range = [body localizedCaseInsensitiveRangeOfString:queryStr];
            if (range.location != NSNotFound) {
                NSInteger start = MAX(0, (NSInteger)range.location - 50);
                NSInteger length = MIN(100, [body length] - start);
                NSString *snippet = [body substringWithRange:NSMakeRange(start, length)];
                result.results[i].snippet = toCString(snippet);
            } else {
                result.results[i].snippet = toCString([body substringToIndex:MIN(100, [body length])]);
            }
            result.results[i].note.body = NULL;
        }

        return result;
    }
}

// Memory cleanup functions
void FreeNote(CNote *note) {
    if (!note) return;
    free((void*)note->id);
    free((void*)note->title);
    free((void*)note->body);
    free((void*)note->folder);
    free(note);
}

void FreeNoteArray(CNoteArray arr) {
    for (int i = 0; i < arr.count; i++) {
        free((void*)arr.notes[i].id);
        free((void*)arr.notes[i].title);
        free((void*)arr.notes[i].body);
        free((void*)arr.notes[i].folder);
    }
    free(arr.notes);
}

void FreeFolderArray(CFolderArray arr) {
    for (int i = 0; i < arr.count; i++) {
        free((void*)arr.folders[i].id);
        free((void*)arr.folders[i].name);
    }
    free(arr.folders);
}

void FreeSearchResultArray(CSearchResultArray arr) {
    for (int i = 0; i < arr.count; i++) {
        free((void*)arr.results[i].note.id);
        free((void*)arr.results[i].note.title);
        free((void*)arr.results[i].note.body);
        free((void*)arr.results[i].note.folder);
        free((void*)arr.results[i].snippet);
    }
    free(arr.results);
}
```

### Go Wrapper

```go
// internal/notes/notes_darwin.go
//go:build darwin

package notes

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework ScriptingBridge
#include "notes_darwin.h"
*/
import "C"
import (
    "errors"
    "time"
    "unsafe"
)

var (
    ErrNotFound    = errors.New("note not found")
    ErrCreateFailed = errors.New("failed to create note")
)

type Client struct{}

func New() NotesClient {
    return &Client{}
}

func (c *Client) Available() bool {
    return C.NotesAvailable() == 1
}

func (c *Client) Create(note Note) (string, error) {
    cTitle := C.CString(note.Title)
    cBody := C.CString(note.Body)
    cFolder := C.CString(note.Folder)
    defer C.free(unsafe.Pointer(cTitle))
    defer C.free(unsafe.Pointer(cBody))
    defer C.free(unsafe.Pointer(cFolder))

    result := C.NotesCreate(cTitle, cBody, cFolder)
    if result == nil {
        return "", ErrCreateFailed
    }
    defer C.free(unsafe.Pointer(result))
    return C.GoString(result), nil
}

func (c *Client) Get(noteID string) (*Note, error) {
    cID := C.CString(noteID)
    defer C.free(unsafe.Pointer(cID))

    cNote := C.NotesGet(cID)
    if cNote == nil {
        return nil, ErrNotFound
    }
    defer C.FreeNote(cNote)

    return &Note{
        ID:         C.GoString(cNote.id),
        Title:      C.GoString(cNote.title),
        Body:       C.GoString(cNote.body),
        Folder:     C.GoString(cNote.folder),
        CreatedAt:  time.Unix(int64(cNote.createdAt), 0),
        ModifiedAt: time.Unix(int64(cNote.modifiedAt), 0),
    }, nil
}

func (c *Client) List(folder string, limit int) ([]Note, error) {
    cFolder := C.CString(folder)
    defer C.free(unsafe.Pointer(cFolder))

    arr := C.NotesListInFolder(cFolder, C.int(limit))
    defer C.FreeNoteArray(arr)

    notes := make([]Note, arr.count)
    cNotes := (*[1 << 20]C.CNote)(unsafe.Pointer(arr.notes))[:arr.count:arr.count]

    for i, cn := range cNotes {
        notes[i] = Note{
            ID:         C.GoString(cn.id),
            Title:      C.GoString(cn.title),
            Folder:     C.GoString(cn.folder),
            CreatedAt:  time.Unix(int64(cn.createdAt), 0),
            ModifiedAt: time.Unix(int64(cn.modifiedAt), 0),
        }
    }
    return notes, nil
}

func (c *Client) Update(noteID string, note Note) error {
    cID := C.CString(noteID)
    cTitle := C.CString(note.Title)
    cBody := C.CString(note.Body)
    defer C.free(unsafe.Pointer(cID))
    defer C.free(unsafe.Pointer(cTitle))
    defer C.free(unsafe.Pointer(cBody))

    if C.NotesUpdate(cID, cTitle, cBody) != 0 {
        return ErrNotFound
    }
    return nil
}

func (c *Client) Delete(noteID string) error {
    cID := C.CString(noteID)
    defer C.free(unsafe.Pointer(cID))

    if C.NotesDelete(cID) != 0 {
        return ErrNotFound
    }
    return nil
}

func (c *Client) ListFolders() ([]Folder, error) {
    arr := C.NotesListFolders()
    defer C.FreeFolderArray(arr)

    folders := make([]Folder, arr.count)
    cFolders := (*[1 << 20]C.CFolder)(unsafe.Pointer(arr.folders))[:arr.count:arr.count]

    for i, cf := range cFolders {
        folders[i] = Folder{
            ID:    C.GoString(cf.id),
            Name:  C.GoString(cf.name),
            Count: int(cf.count),
        }
    }
    return folders, nil
}

func (c *Client) CreateFolder(name string) error {
    cName := C.CString(name)
    defer C.free(unsafe.Pointer(cName))

    if C.NotesCreateFolder(cName) != 0 {
        return errors.New("failed to create folder")
    }
    return nil
}

func (c *Client) DeleteFolder(name string) error {
    cName := C.CString(name)
    defer C.free(unsafe.Pointer(cName))

    if C.NotesDeleteFolder(cName) != 0 {
        return ErrNotFound
    }
    return nil
}

func (c *Client) Search(query string, folder string) ([]SearchResult, error) {
    cQuery := C.CString(query)
    var cFolder *C.char
    if folder != "" {
        cFolder = C.CString(folder)
        defer C.free(unsafe.Pointer(cFolder))
    }
    defer C.free(unsafe.Pointer(cQuery))

    arr := C.NotesSearch(cQuery, cFolder)
    defer C.FreeSearchResultArray(arr)

    results := make([]SearchResult, arr.count)
    cResults := (*[1 << 20]C.CSearchResult)(unsafe.Pointer(arr.results))[:arr.count:arr.count]

    for i, cr := range cResults {
        results[i] = SearchResult{
            Note: Note{
                ID:         C.GoString(cr.note.id),
                Title:      C.GoString(cr.note.title),
                Folder:     C.GoString(cr.note.folder),
                CreatedAt:  time.Unix(int64(cr.note.createdAt), 0),
                ModifiedAt: time.Unix(int64(cr.note.modifiedAt), 0),
            },
            Snippet: C.GoString(cr.snippet),
        }
    }
    return results, nil
}
```

## Stub Implementation (Non-macOS)

```go
// internal/notes/stub_other.go
//go:build !darwin

package notes

import "errors"

var ErrNotSupported = errors.New("Apple Notes integration is only available on macOS")

type stubClient struct{}

func New() NotesClient                                              { return &stubClient{} }
func (c *stubClient) Available() bool                               { return false }
func (c *stubClient) Create(note Note) (string, error)              { return "", ErrNotSupported }
func (c *stubClient) Get(noteID string) (*Note, error)              { return nil, ErrNotSupported }
func (c *stubClient) List(folder string, limit int) ([]Note, error) { return nil, ErrNotSupported }
func (c *stubClient) Update(noteID string, note Note) error         { return ErrNotSupported }
func (c *stubClient) Delete(noteID string) error                    { return ErrNotSupported }
func (c *stubClient) ListFolders() ([]Folder, error)                { return nil, ErrNotSupported }
func (c *stubClient) CreateFolder(name string) error                { return ErrNotSupported }
func (c *stubClient) DeleteFolder(name string) error                { return ErrNotSupported }
func (c *stubClient) Search(query, folder string) ([]SearchResult, error) { return nil, ErrNotSupported }
```

## UI Flow

### Key Bindings

| Key | Context | Action |
|-----|---------|--------|
| `N` | Email read view | Save email to Notes |
| `Ctrl+N` | Any view | Open Notes browser |

### Notes Browser View

New TUI view for browsing/managing notes:

```
┌─ Notes ─────────────────────────────────────────────────────┐
│ Folder: Maily (12 notes)                    [/] Search      │
├─────────────────────────────────────────────────────────────┤
│ > Meeting notes from Q4 review              Jan 14, 2026    │
│   Project kickoff action items              Jan 13, 2026    │
│   Client feedback summary                   Jan 12, 2026    │
│   Weekly standup notes                      Jan 10, 2026    │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│ [n]ew  [e]dit  [d]elete  [f]older  [/]search  [q]uit       │
└─────────────────────────────────────────────────────────────┘
```

### Save Email Dialog

When pressing `N` in email read view:

```
┌─────────────────────────────────────┐
│       Save to Notes                 │
├─────────────────────────────────────┤
│  Folder: [Maily          ▼]        │
│                                     │
│  [1] Save full email                │
│  [2] Save with AI summary           │
│  [3] Cancel                         │
└─────────────────────────────────────┘
```

### Note Detail View

```
┌─ Note ──────────────────────────────────────────────────────┐
│ Meeting notes from Q4 review                                │
│ Folder: Maily | Created: Jan 14, 2026 | Modified: Jan 14    │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│ From: john@example.com                                      │
│ Date: January 14, 2026                                      │
│ Subject: Meeting notes from Q4 review                       │
│                                                             │
│ ─────────────────────────────────────                       │
│                                                             │
│ Key discussion points:                                      │
│ - Revenue exceeded targets by 15%                           │
│ - New product launch scheduled for Q2                       │
│ ...                                                         │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│ [e]dit  [d]elete  [Esc] back                               │
└─────────────────────────────────────────────────────────────┘
```

### Search View

```
┌─ Search Notes ──────────────────────────────────────────────┐
│ Query: quarterly review                                     │
├─────────────────────────────────────────────────────────────┤
│ Found 3 notes:                                              │
│                                                             │
│ > Meeting notes from Q4 review                              │
│   "...discussed quarterly review process and..."            │
│                                                             │
│   Q3 Quarterly Planning                                     │
│   "...review of quarterly goals showed..."                  │
│                                                             │
│   Team Review Notes                                         │
│   "...quarterly performance review feedback..."             │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│ [Enter] open  [Esc] cancel                                  │
└─────────────────────────────────────────────────────────────┘
```

### Edit Note View

```
┌─ Edit Note ─────────────────────────────────────────────────┐
│ Title: [Meeting notes from Q4 review                    ]   │
├─────────────────────────────────────────────────────────────┤
│ │Key discussion points:                                     │
│ │- Revenue exceeded targets by 15%                          │
│ │- New product launch scheduled for Q2                      │
│ │- Team expansion approved for engineering                  │
│ │                                                           │
│ │Action items:                                              │
│ │- [ ] Prepare Q2 roadmap by Jan 20                        │
│ │- [ ] Schedule hiring kickoff                              │
│ │                                                           │
├─────────────────────────────────────────────────────────────┤
│ [Ctrl+S] save  [Esc] cancel                                │
└─────────────────────────────────────────────────────────────┘
```

## Integration Points

### Files to Create

| File | Purpose |
|------|---------|
| `internal/notes/notes.go` | Interface and types |
| `internal/notes/notes_darwin.h` | C header for CGO |
| `internal/notes/notes_darwin.m` | Objective-C ScriptingBridge implementation |
| `internal/notes/notes_darwin.go` | Go wrapper for CGO |
| `internal/notes/stub_other.go` | Non-macOS stub |

### Files to Modify

| File | Changes |
|------|---------|
| `internal/ui/app.go` | Add Notes states, key handlers |
| `internal/ui/commands.go` | Add notes CRUD commands |
| `internal/ui/notes.go` | New file: Notes browser view |
| `internal/ui/notes_edit.go` | New file: Note editor view |
| `internal/cli/notes.go` | New file: CLI `maily notes` command |
| `config/config.go` | Add `notes.folder` config |

### App States

```go
const (
    // ... existing states
    stateNotesBrowser       // Notes list view
    stateNoteDetail         // Single note view
    stateNoteEdit           // Edit note
    stateNotesSearch        // Search results
    stateSaveToNotesDialog  // Save email dialog
    stateSavingToNotes      // Spinner while saving
)
```

### CLI Commands

```bash
maily notes                    # Open Notes browser TUI
maily notes list [--folder]    # List notes
maily notes search <query>     # Search notes
maily notes show <id>          # Show note content
maily notes delete <id>        # Delete note
```

## Note Format

When saving emails to Notes:

```
From: sender@example.com
Date: January 14, 2026
Subject: Meeting notes from Q4 review

───────────────────────────────────────

[Email body content or AI summary]
```

## Configuration

```yaml
# ~/.config/maily/config.yml
notes:
  folder: "Maily"           # Default folder for saved emails
  show_in_menu: true        # Show Notes in command palette
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Non-macOS platform | Show "Not available on this platform" |
| Notes.app not running | ScriptingBridge auto-launches it |
| Note not found | Show "Note not found or was deleted" |
| Folder not found | Auto-create folder |
| Search returns empty | Show "No notes match your search" |
| Permission denied | Show "Grant Maily access in System Settings > Privacy > Automation" |

## Performance

ScriptingBridge advantages over osascript:

| Metric | osascript | ScriptingBridge |
|--------|-----------|-----------------|
| Process spawn | 1 per call | None (in-process) |
| Latency | ~100-200ms | ~10-20ms |
| Memory | New process each time | Shared |
| Parsing | String parsing | Native objects |

## Testing

### Manual Testing

1. **Create**: Save email to Notes, verify in Notes.app
2. **Read**: Open Notes browser, verify list matches Notes.app
3. **Update**: Edit note, verify changes in Notes.app
4. **Delete**: Delete note, verify removed from Notes.app
5. **Search**: Search for term, verify results match Notes.app search

### Automated Testing

- Unit tests for Go wrapper logic
- Integration tests require macOS runner
- Mock client for non-macOS CI

## Future Enhancements

1. **Attachments**: Save email attachments to note
2. **Rich HTML**: Preserve email formatting
3. **Tags**: Support Notes.app tags (macOS 14+)
4. **Shared folders**: Support iCloud shared folders
5. **Sync indicator**: Show sync status with iCloud
6. **Quick note**: Global hotkey to create note from anywhere
