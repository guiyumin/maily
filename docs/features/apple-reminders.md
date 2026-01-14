# Apple Reminders Integration

Create tasks from emails in macOS Reminders via EventKit (same framework as Calendar).

## Overview

This feature allows users to create actionable tasks from emails:
- **Create**: Turn email into a reminder/task
- **Read**: Browse reminders within Maily
- **Update**: Edit reminder details
- **Delete**: Remove reminders
- **Complete**: Mark tasks as done
- **Search**: Find reminders

Complements Calendar (scheduled events) and Notes (reference material).

## Why EventKit (not ScriptingBridge)

Reminders uses **EventKit**, the same framework as Calendar:

```
EventKit Framework
├── EKEventStore    # Main access point (shared)
├── EKCalendar      # Calendar/Reminder lists
├── EKEvent         # Calendar events (existing)
└── EKReminder      # Reminders/tasks (this feature)
```

| Approach | Pros | Cons |
|----------|------|------|
| **EventKit** | Native API, same as Calendar, fast, full features | CGO required (already have it) |
| **ScriptingBridge** | Works | Unnecessary, slower, less features |

## Use Cases

| Email Scenario | Reminder |
|----------------|----------|
| "Can you review this by Friday?" | "Review doc for John" due Friday |
| "Let's circle back next week" | "Follow up with Sarah" due next Monday |
| "Invoice attached" | "Pay invoice #1234" |
| "Interested in your product" | "Reply to lead: Acme Corp" |
| Meeting follow-up needed | "Send meeting notes to team" |

## Architecture

```
internal/reminders/
├── reminders.go           # Abstract interface
├── eventkit_darwin.go     # macOS implementation (EventKit via CGO)
├── eventkit_darwin.m      # Objective-C EventKit implementation
├── eventkit_darwin.h      # Objective-C header
└── stub_other.go          # Stub for non-macOS
```

### Data Types

```go
// internal/reminders/reminders.go
package reminders

import "time"

// Priority levels
const (
    PriorityNone   = 0
    PriorityLow    = 9
    PriorityMedium = 5
    PriorityHigh   = 1
)

// Reminder represents a task
type Reminder struct {
    ID          string
    Title       string
    Notes       string     // Body/description
    DueDate     *time.Time // Optional due date
    Priority    int        // 0=none, 1=high, 5=medium, 9=low
    Completed   bool
    CompletedAt *time.Time
    List        string     // List/folder name
    CreatedAt   time.Time
    ModifiedAt  time.Time
}

// List represents a Reminders list
type List struct {
    ID    string
    Name  string
    Color string // Hex color
    Count int    // Number of reminders
}
```

### Interface

```go
type RemindersClient interface {
    // Availability
    Available() bool

    // CRUD - Reminders
    Create(reminder Reminder) (string, error)
    Get(reminderID string) (*Reminder, error)
    List(listName string, includeCompleted bool) ([]Reminder, error)
    Update(reminderID string, reminder Reminder) error
    Delete(reminderID string) error
    Complete(reminderID string) error
    Uncomplete(reminderID string) error

    // Lists
    ListLists() ([]List, error)
    CreateList(name string) error
    DeleteList(name string) error

    // Search
    Search(query string, listName string) ([]Reminder, error)
}

func New() RemindersClient
```

## macOS Implementation (EventKit)

Uses the same EventKit framework as Calendar, with `EKReminder` class.

### Header File

```objc
// internal/reminders/eventkit_darwin.h
#ifndef REMINDERS_EVENTKIT_H
#define REMINDERS_EVENTKIT_H

#import <Foundation/Foundation.h>

typedef struct {
    const char *id;
    const char *title;
    const char *notes;
    double dueDate;      // Unix timestamp, 0 if not set
    int priority;        // 0=none, 1=high, 5=medium, 9=low
    int completed;       // 0 or 1
    double completedAt;  // Unix timestamp
    const char *list;
    double createdAt;
    double modifiedAt;
} CReminder;

typedef struct {
    const char *id;
    const char *name;
    const char *color;   // Hex color
    int count;
} CReminderList;

typedef struct {
    CReminder *reminders;
    int count;
} CReminderArray;

typedef struct {
    CReminderList *lists;
    int count;
} CReminderListArray;

// Permission
int RemindersRequestAccess(void);
int RemindersAvailable(void);

// CRUD - Reminders
const char* RemindersCreate(const char *title, const char *notes, double dueDate, int priority, const char *listID);
CReminder* RemindersGet(const char *reminderID);
CReminderArray RemindersListInCalendar(const char *listID, int includeCompleted);
int RemindersUpdate(const char *reminderID, const char *title, const char *notes, double dueDate, int priority);
int RemindersDelete(const char *reminderID);
int RemindersComplete(const char *reminderID);
int RemindersUncomplete(const char *reminderID);

// Lists (EKCalendar with type = reminder)
CReminderListArray RemindersGetLists(void);
const char* RemindersCreateList(const char *name);
int RemindersDeleteList(const char *listID);

// Search
CReminderArray RemindersSearch(const char *query, const char *listID);

// Memory cleanup
void FreeReminder(CReminder *reminder);
void FreeReminderArray(CReminderArray arr);
void FreeReminderListArray(CReminderListArray arr);

#endif
```

### Objective-C Implementation

```objc
// internal/reminders/eventkit_darwin.m
#import "eventkit_darwin.h"
#import <EventKit/EventKit.h>

static EKEventStore *eventStore = nil;
static BOOL accessGranted = NO;

static EKEventStore* getEventStore(void) {
    static dispatch_once_t onceToken;
    dispatch_once(&onceToken, ^{
        eventStore = [[EKEventStore alloc] init];
    });
    return eventStore;
}

static const char* toCString(NSString *str) {
    if (!str) return NULL;
    return strdup([str UTF8String]);
}

static NSString* colorToHex(CGColorRef color) {
    if (!color) return nil;
    const CGFloat *components = CGColorGetComponents(color);
    size_t count = CGColorGetNumberOfComponents(color);
    if (count >= 3) {
        return [NSString stringWithFormat:@"#%02X%02X%02X",
            (int)(components[0] * 255),
            (int)(components[1] * 255),
            (int)(components[2] * 255)];
    }
    return nil;
}

int RemindersRequestAccess(void) {
    EKEventStore *store = getEventStore();
    dispatch_semaphore_t sema = dispatch_semaphore_create(0);

    if (@available(macOS 14.0, *)) {
        [store requestFullAccessToRemindersWithCompletion:^(BOOL granted, NSError *error) {
            accessGranted = granted;
            dispatch_semaphore_signal(sema);
        }];
    } else {
        [store requestAccessToEntityType:EKEntityTypeReminder completion:^(BOOL granted, NSError *error) {
            accessGranted = granted;
            dispatch_semaphore_signal(sema);
        }];
    }

    dispatch_semaphore_wait(sema, DISPATCH_TIME_FOREVER);
    return accessGranted ? 1 : 0;
}

int RemindersAvailable(void) {
    EKAuthorizationStatus status = [EKEventStore authorizationStatusForEntityType:EKEntityTypeReminder];
    return (status == EKAuthorizationStatusAuthorized) ? 1 : 0;
}

const char* RemindersCreate(const char *title, const char *notes, double dueDate, int priority, const char *listID) {
    @autoreleasepool {
        EKEventStore *store = getEventStore();
        if (!store) return NULL;

        EKReminder *reminder = [EKReminder reminderWithEventStore:store];
        reminder.title = [NSString stringWithUTF8String:title];

        if (notes) {
            reminder.notes = [NSString stringWithUTF8String:notes];
        }

        // Set due date
        if (dueDate > 0) {
            NSDate *date = [NSDate dateWithTimeIntervalSince1970:dueDate];
            NSCalendar *calendar = [NSCalendar currentCalendar];
            reminder.dueDateComponents = [calendar componentsInTimeZone:[NSTimeZone localTimeZone]
                                                               fromDate:date];
        }

        reminder.priority = priority;

        // Find calendar (list)
        EKCalendar *targetCalendar = nil;
        if (listID) {
            NSString *lid = [NSString stringWithUTF8String:listID];
            targetCalendar = [store calendarWithIdentifier:lid];
        }
        if (!targetCalendar) {
            targetCalendar = [store defaultCalendarForNewReminders];
        }
        reminder.calendar = targetCalendar;

        NSError *error = nil;
        BOOL success = [store saveReminder:reminder commit:YES error:&error];
        if (!success || error) {
            return NULL;
        }

        return toCString(reminder.calendarItemIdentifier);
    }
}

CReminder* RemindersGet(const char *reminderID) {
    @autoreleasepool {
        EKEventStore *store = getEventStore();
        if (!store) return NULL;

        NSString *rid = [NSString stringWithUTF8String:reminderID];
        EKCalendarItem *item = [store calendarItemWithIdentifier:rid];

        if (!item || ![item isKindOfClass:[EKReminder class]]) {
            return NULL;
        }

        EKReminder *r = (EKReminder *)item;
        CReminder *result = malloc(sizeof(CReminder));
        result->id = toCString(r.calendarItemIdentifier);
        result->title = toCString(r.title);
        result->notes = toCString(r.notes);
        result->priority = (int)r.priority;
        result->completed = r.completed ? 1 : 0;
        result->list = toCString(r.calendar.calendarIdentifier);

        if (r.dueDateComponents) {
            NSDate *due = [[NSCalendar currentCalendar] dateFromComponents:r.dueDateComponents];
            result->dueDate = due ? [due timeIntervalSince1970] : 0;
        } else {
            result->dueDate = 0;
        }

        result->completedAt = r.completionDate ? [r.completionDate timeIntervalSince1970] : 0;
        result->createdAt = r.creationDate ? [r.creationDate timeIntervalSince1970] : 0;
        result->modifiedAt = r.lastModifiedDate ? [r.lastModifiedDate timeIntervalSince1970] : 0;

        return result;
    }
}

CReminderArray RemindersListInCalendar(const char *listID, int includeCompleted) {
    @autoreleasepool {
        EKEventStore *store = getEventStore();
        CReminderArray result = {NULL, 0};
        if (!store) return result;

        // Find calendar
        EKCalendar *calendar = nil;
        if (listID) {
            calendar = [store calendarWithIdentifier:[NSString stringWithUTF8String:listID]];
        }
        if (!calendar) {
            calendar = [store defaultCalendarForNewReminders];
        }
        if (!calendar) return result;

        // Create predicate for reminders
        NSPredicate *predicate = [store predicateForRemindersInCalendars:@[calendar]];

        // Fetch reminders synchronously
        dispatch_semaphore_t sema = dispatch_semaphore_create(0);
        __block NSArray<EKReminder *> *reminders = nil;

        [store fetchRemindersMatchingPredicate:predicate completion:^(NSArray<EKReminder *> *fetchedReminders) {
            reminders = fetchedReminders;
            dispatch_semaphore_signal(sema);
        }];

        dispatch_semaphore_wait(sema, DISPATCH_TIME_FOREVER);

        if (!reminders) return result;

        // Filter completed if needed
        NSMutableArray *filtered = [NSMutableArray array];
        for (EKReminder *r in reminders) {
            if (includeCompleted || !r.completed) {
                [filtered addObject:r];
            }
        }

        result.count = (int)[filtered count];
        result.reminders = malloc(sizeof(CReminder) * result.count);

        for (int i = 0; i < result.count; i++) {
            EKReminder *r = filtered[i];
            result.reminders[i].id = toCString(r.calendarItemIdentifier);
            result.reminders[i].title = toCString(r.title);
            result.reminders[i].notes = NULL; // Skip for performance
            result.reminders[i].priority = (int)r.priority;
            result.reminders[i].completed = r.completed ? 1 : 0;
            result.reminders[i].list = toCString(r.calendar.calendarIdentifier);

            if (r.dueDateComponents) {
                NSDate *due = [[NSCalendar currentCalendar] dateFromComponents:r.dueDateComponents];
                result.reminders[i].dueDate = due ? [due timeIntervalSince1970] : 0;
            } else {
                result.reminders[i].dueDate = 0;
            }

            result.reminders[i].completedAt = r.completionDate ? [r.completionDate timeIntervalSince1970] : 0;
            result.reminders[i].createdAt = r.creationDate ? [r.creationDate timeIntervalSince1970] : 0;
            result.reminders[i].modifiedAt = r.lastModifiedDate ? [r.lastModifiedDate timeIntervalSince1970] : 0;
        }

        return result;
    }
}

int RemindersUpdate(const char *reminderID, const char *title, const char *notes, double dueDate, int priority) {
    @autoreleasepool {
        EKEventStore *store = getEventStore();
        if (!store) return -1;

        NSString *rid = [NSString stringWithUTF8String:reminderID];
        EKCalendarItem *item = [store calendarItemWithIdentifier:rid];

        if (!item || ![item isKindOfClass:[EKReminder class]]) {
            return -1;
        }

        EKReminder *r = (EKReminder *)item;

        if (title) r.title = [NSString stringWithUTF8String:title];
        if (notes) r.notes = [NSString stringWithUTF8String:notes];

        if (dueDate > 0) {
            NSDate *date = [NSDate dateWithTimeIntervalSince1970:dueDate];
            NSCalendar *calendar = [NSCalendar currentCalendar];
            r.dueDateComponents = [calendar componentsInTimeZone:[NSTimeZone localTimeZone] fromDate:date];
        } else if (dueDate == -1) {
            r.dueDateComponents = nil; // Clear due date
        }

        r.priority = priority;

        NSError *error = nil;
        BOOL success = [store saveReminder:r commit:YES error:&error];
        return (success && !error) ? 0 : -1;
    }
}

int RemindersDelete(const char *reminderID) {
    @autoreleasepool {
        EKEventStore *store = getEventStore();
        if (!store) return -1;

        NSString *rid = [NSString stringWithUTF8String:reminderID];
        EKCalendarItem *item = [store calendarItemWithIdentifier:rid];

        if (!item || ![item isKindOfClass:[EKReminder class]]) {
            return -1;
        }

        NSError *error = nil;
        BOOL success = [store removeReminder:(EKReminder *)item commit:YES error:&error];
        return (success && !error) ? 0 : -1;
    }
}

int RemindersComplete(const char *reminderID) {
    @autoreleasepool {
        EKEventStore *store = getEventStore();
        if (!store) return -1;

        NSString *rid = [NSString stringWithUTF8String:reminderID];
        EKCalendarItem *item = [store calendarItemWithIdentifier:rid];

        if (!item || ![item isKindOfClass:[EKReminder class]]) {
            return -1;
        }

        EKReminder *r = (EKReminder *)item;
        r.completed = YES;

        NSError *error = nil;
        BOOL success = [store saveReminder:r commit:YES error:&error];
        return (success && !error) ? 0 : -1;
    }
}

int RemindersUncomplete(const char *reminderID) {
    @autoreleasepool {
        EKEventStore *store = getEventStore();
        if (!store) return -1;

        NSString *rid = [NSString stringWithUTF8String:reminderID];
        EKCalendarItem *item = [store calendarItemWithIdentifier:rid];

        if (!item || ![item isKindOfClass:[EKReminder class]]) {
            return -1;
        }

        EKReminder *r = (EKReminder *)item;
        r.completed = NO;

        NSError *error = nil;
        BOOL success = [store saveReminder:r commit:YES error:&error];
        return (success && !error) ? 0 : -1;
    }
}

CReminderListArray RemindersGetLists(void) {
    @autoreleasepool {
        EKEventStore *store = getEventStore();
        CReminderListArray result = {NULL, 0};
        if (!store) return result;

        NSArray<EKCalendar *> *calendars = [store calendarsForEntityType:EKEntityTypeReminder];
        result.count = (int)[calendars count];
        result.lists = malloc(sizeof(CReminderList) * result.count);

        for (int i = 0; i < result.count; i++) {
            EKCalendar *cal = calendars[i];
            result.lists[i].id = toCString(cal.calendarIdentifier);
            result.lists[i].name = toCString(cal.title);
            result.lists[i].color = toCString(colorToHex(cal.CGColor));

            // Count incomplete reminders (expensive, could optimize)
            NSPredicate *pred = [store predicateForRemindersInCalendars:@[cal]];
            dispatch_semaphore_t sema = dispatch_semaphore_create(0);
            __block int count = 0;

            [store fetchRemindersMatchingPredicate:pred completion:^(NSArray<EKReminder *> *reminders) {
                for (EKReminder *r in reminders) {
                    if (!r.completed) count++;
                }
                dispatch_semaphore_signal(sema);
            }];

            dispatch_semaphore_wait(sema, DISPATCH_TIME_FOREVER);
            result.lists[i].count = count;
        }

        return result;
    }
}

const char* RemindersCreateList(const char *name) {
    @autoreleasepool {
        EKEventStore *store = getEventStore();
        if (!store) return NULL;

        // Find a local source for reminders
        EKSource *localSource = nil;
        for (EKSource *source in store.sources) {
            if (source.sourceType == EKSourceTypeLocal ||
                source.sourceType == EKSourceTypeCalDAV) {
                localSource = source;
                break;
            }
        }
        if (!localSource) return NULL;

        EKCalendar *newList = [EKCalendar calendarForEntityType:EKEntityTypeReminder eventStore:store];
        newList.title = [NSString stringWithUTF8String:name];
        newList.source = localSource;

        NSError *error = nil;
        BOOL success = [store saveCalendar:newList commit:YES error:&error];
        if (!success || error) {
            return NULL;
        }

        return toCString(newList.calendarIdentifier);
    }
}

int RemindersDeleteList(const char *listID) {
    @autoreleasepool {
        EKEventStore *store = getEventStore();
        if (!store) return -1;

        EKCalendar *calendar = [store calendarWithIdentifier:[NSString stringWithUTF8String:listID]];
        if (!calendar) return -1;

        NSError *error = nil;
        BOOL success = [store removeCalendar:calendar commit:YES error:&error];
        return (success && !error) ? 0 : -1;
    }
}

CReminderArray RemindersSearch(const char *query, const char *listID) {
    @autoreleasepool {
        EKEventStore *store = getEventStore();
        CReminderArray result = {NULL, 0};
        if (!store) return result;

        NSString *queryStr = [NSString stringWithUTF8String:query];

        // Get calendars to search
        NSArray<EKCalendar *> *calendars;
        if (listID) {
            EKCalendar *cal = [store calendarWithIdentifier:[NSString stringWithUTF8String:listID]];
            calendars = cal ? @[cal] : @[];
        } else {
            calendars = [store calendarsForEntityType:EKEntityTypeReminder];
        }

        if ([calendars count] == 0) return result;

        // Fetch all reminders
        NSPredicate *predicate = [store predicateForRemindersInCalendars:calendars];
        dispatch_semaphore_t sema = dispatch_semaphore_create(0);
        __block NSArray<EKReminder *> *allReminders = nil;

        [store fetchRemindersMatchingPredicate:predicate completion:^(NSArray<EKReminder *> *reminders) {
            allReminders = reminders;
            dispatch_semaphore_signal(sema);
        }];

        dispatch_semaphore_wait(sema, DISPATCH_TIME_FOREVER);

        // Filter by query
        NSMutableArray *matches = [NSMutableArray array];
        for (EKReminder *r in allReminders) {
            NSString *title = r.title ?: @"";
            NSString *notes = r.notes ?: @"";

            if ([title localizedCaseInsensitiveContainsString:queryStr] ||
                [notes localizedCaseInsensitiveContainsString:queryStr]) {
                [matches addObject:r];
            }
        }

        result.count = (int)[matches count];
        result.reminders = malloc(sizeof(CReminder) * result.count);

        for (int i = 0; i < result.count; i++) {
            EKReminder *r = matches[i];
            result.reminders[i].id = toCString(r.calendarItemIdentifier);
            result.reminders[i].title = toCString(r.title);
            result.reminders[i].notes = toCString(r.notes);
            result.reminders[i].priority = (int)r.priority;
            result.reminders[i].completed = r.completed ? 1 : 0;
            result.reminders[i].list = toCString(r.calendar.calendarIdentifier);

            if (r.dueDateComponents) {
                NSDate *due = [[NSCalendar currentCalendar] dateFromComponents:r.dueDateComponents];
                result.reminders[i].dueDate = due ? [due timeIntervalSince1970] : 0;
            } else {
                result.reminders[i].dueDate = 0;
            }

            result.reminders[i].completedAt = r.completionDate ? [r.completionDate timeIntervalSince1970] : 0;
            result.reminders[i].createdAt = r.creationDate ? [r.creationDate timeIntervalSince1970] : 0;
            result.reminders[i].modifiedAt = r.lastModifiedDate ? [r.lastModifiedDate timeIntervalSince1970] : 0;
        }

        return result;
    }
}

void FreeReminder(CReminder *reminder) {
    if (!reminder) return;
    free((void*)reminder->id);
    free((void*)reminder->title);
    free((void*)reminder->notes);
    free((void*)reminder->list);
    free(reminder);
}

void FreeReminderArray(CReminderArray arr) {
    for (int i = 0; i < arr.count; i++) {
        free((void*)arr.reminders[i].id);
        free((void*)arr.reminders[i].title);
        free((void*)arr.reminders[i].notes);
        free((void*)arr.reminders[i].list);
    }
    free(arr.reminders);
}

void FreeReminderListArray(CReminderListArray arr) {
    for (int i = 0; i < arr.count; i++) {
        free((void*)arr.lists[i].id);
        free((void*)arr.lists[i].name);
        free((void*)arr.lists[i].color);
    }
    free(arr.lists);
}
```

### Go Wrapper

```go
// internal/reminders/eventkit_darwin.go
//go:build darwin

package reminders

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Foundation -framework EventKit
#include "eventkit_darwin.h"
*/
import "C"
import (
    "errors"
    "time"
    "unsafe"
)

var (
    ErrNotFound     = errors.New("reminder not found")
    ErrCreateFailed = errors.New("failed to create reminder")
    ErrNoAccess     = errors.New("no access to reminders")
)

type Client struct{}

func New() RemindersClient {
    return &Client{}
}

func (c *Client) RequestAccess() bool {
    return C.RemindersRequestAccess() == 1
}

func (c *Client) Available() bool {
    return C.RemindersAvailable() == 1
}

func (c *Client) Create(reminder Reminder) (string, error) {
    cTitle := C.CString(reminder.Title)
    defer C.free(unsafe.Pointer(cTitle))

    var cNotes *C.char
    if reminder.Notes != "" {
        cNotes = C.CString(reminder.Notes)
        defer C.free(unsafe.Pointer(cNotes))
    }

    var dueDate C.double
    if reminder.DueDate != nil {
        dueDate = C.double(reminder.DueDate.Unix())
    }

    var cList *C.char
    if reminder.List != "" {
        cList = C.CString(reminder.List)
        defer C.free(unsafe.Pointer(cList))
    }

    result := C.RemindersCreate(cTitle, cNotes, dueDate, C.int(reminder.Priority), cList)
    if result == nil {
        return "", ErrCreateFailed
    }
    defer C.free(unsafe.Pointer(result))
    return C.GoString(result), nil
}

func (c *Client) Get(reminderID string) (*Reminder, error) {
    cID := C.CString(reminderID)
    defer C.free(unsafe.Pointer(cID))

    cReminder := C.RemindersGet(cID)
    if cReminder == nil {
        return nil, ErrNotFound
    }
    defer C.FreeReminder(cReminder)

    return cReminderToGo(cReminder), nil
}

func (c *Client) List(listID string, includeCompleted bool) ([]Reminder, error) {
    var cList *C.char
    if listID != "" {
        cList = C.CString(listID)
        defer C.free(unsafe.Pointer(cList))
    }

    include := 0
    if includeCompleted {
        include = 1
    }

    arr := C.RemindersListInCalendar(cList, C.int(include))
    defer C.FreeReminderArray(arr)

    return cReminderArrayToGo(arr), nil
}

func (c *Client) Update(reminderID string, reminder Reminder) error {
    cID := C.CString(reminderID)
    cTitle := C.CString(reminder.Title)
    defer C.free(unsafe.Pointer(cID))
    defer C.free(unsafe.Pointer(cTitle))

    var cNotes *C.char
    if reminder.Notes != "" {
        cNotes = C.CString(reminder.Notes)
        defer C.free(unsafe.Pointer(cNotes))
    }

    var dueDate C.double = -1 // -1 means clear
    if reminder.DueDate != nil {
        dueDate = C.double(reminder.DueDate.Unix())
    }

    if C.RemindersUpdate(cID, cTitle, cNotes, dueDate, C.int(reminder.Priority)) != 0 {
        return ErrNotFound
    }
    return nil
}

func (c *Client) Delete(reminderID string) error {
    cID := C.CString(reminderID)
    defer C.free(unsafe.Pointer(cID))

    if C.RemindersDelete(cID) != 0 {
        return ErrNotFound
    }
    return nil
}

func (c *Client) Complete(reminderID string) error {
    cID := C.CString(reminderID)
    defer C.free(unsafe.Pointer(cID))

    if C.RemindersComplete(cID) != 0 {
        return ErrNotFound
    }
    return nil
}

func (c *Client) Uncomplete(reminderID string) error {
    cID := C.CString(reminderID)
    defer C.free(unsafe.Pointer(cID))

    if C.RemindersUncomplete(cID) != 0 {
        return ErrNotFound
    }
    return nil
}

func (c *Client) ListLists() ([]List, error) {
    arr := C.RemindersGetLists()
    defer C.FreeReminderListArray(arr)

    lists := make([]List, arr.count)
    cLists := (*[1 << 20]C.CReminderList)(unsafe.Pointer(arr.lists))[:arr.count:arr.count]

    for i, cl := range cLists {
        lists[i] = List{
            ID:    C.GoString(cl.id),
            Name:  C.GoString(cl.name),
            Color: C.GoString(cl.color),
            Count: int(cl.count),
        }
    }
    return lists, nil
}

func (c *Client) CreateList(name string) (string, error) {
    cName := C.CString(name)
    defer C.free(unsafe.Pointer(cName))

    result := C.RemindersCreateList(cName)
    if result == nil {
        return "", errors.New("failed to create list")
    }
    defer C.free(unsafe.Pointer(result))
    return C.GoString(result), nil
}

func (c *Client) DeleteList(listID string) error {
    cID := C.CString(listID)
    defer C.free(unsafe.Pointer(cID))

    if C.RemindersDeleteList(cID) != 0 {
        return ErrNotFound
    }
    return nil
}

func (c *Client) Search(query string, listID string) ([]Reminder, error) {
    cQuery := C.CString(query)
    defer C.free(unsafe.Pointer(cQuery))

    var cList *C.char
    if listID != "" {
        cList = C.CString(listID)
        defer C.free(unsafe.Pointer(cList))
    }

    arr := C.RemindersSearch(cQuery, cList)
    defer C.FreeReminderArray(arr)

    return cReminderArrayToGo(arr), nil
}

// Helper functions
func cReminderToGo(cr *C.CReminder) *Reminder {
    r := &Reminder{
        ID:         C.GoString(cr.id),
        Title:      C.GoString(cr.title),
        Notes:      C.GoString(cr.notes),
        Priority:   int(cr.priority),
        Completed:  cr.completed == 1,
        List:       C.GoString(cr.list),
        CreatedAt:  time.Unix(int64(cr.createdAt), 0),
        ModifiedAt: time.Unix(int64(cr.modifiedAt), 0),
    }
    if cr.dueDate > 0 {
        t := time.Unix(int64(cr.dueDate), 0)
        r.DueDate = &t
    }
    if cr.completedAt > 0 {
        t := time.Unix(int64(cr.completedAt), 0)
        r.CompletedAt = &t
    }
    return r
}

func cReminderArrayToGo(arr C.CReminderArray) []Reminder {
    if arr.count == 0 || arr.reminders == nil {
        return []Reminder{}
    }

    reminders := make([]Reminder, arr.count)
    cReminders := (*[1 << 20]C.CReminder)(unsafe.Pointer(arr.reminders))[:arr.count:arr.count]

    for i, cr := range cReminders {
        reminders[i] = Reminder{
            ID:         C.GoString(cr.id),
            Title:      C.GoString(cr.title),
            Priority:   int(cr.priority),
            Completed:  cr.completed == 1,
            List:       C.GoString(cr.list),
            CreatedAt:  time.Unix(int64(cr.createdAt), 0),
            ModifiedAt: time.Unix(int64(cr.modifiedAt), 0),
        }
        if cr.dueDate > 0 {
            t := time.Unix(int64(cr.dueDate), 0)
            reminders[i].DueDate = &t
        }
    }
    return reminders
}
```

## Stub Implementation (Non-macOS)

```go
// internal/reminders/stub_other.go
//go:build !darwin

package reminders

import "errors"

var ErrNotSupported = errors.New("Apple Reminders integration is only available on macOS")

type stubClient struct{}

func New() RemindersClient                                              { return &stubClient{} }
func (c *stubClient) Available() bool                                   { return false }
func (c *stubClient) Create(reminder Reminder) (string, error)          { return "", ErrNotSupported }
func (c *stubClient) Get(reminderID string) (*Reminder, error)          { return nil, ErrNotSupported }
func (c *stubClient) List(listName string, includeCompleted bool) ([]Reminder, error) { return nil, ErrNotSupported }
func (c *stubClient) Update(reminderID string, reminder Reminder) error { return ErrNotSupported }
func (c *stubClient) Delete(reminderID string) error                    { return ErrNotSupported }
func (c *stubClient) Complete(reminderID string) error                  { return ErrNotSupported }
func (c *stubClient) Uncomplete(reminderID string) error                { return ErrNotSupported }
func (c *stubClient) ListLists() ([]List, error)                        { return nil, ErrNotSupported }
func (c *stubClient) CreateList(name string) error                      { return ErrNotSupported }
func (c *stubClient) DeleteList(name string) error                      { return ErrNotSupported }
func (c *stubClient) Search(query, listName string) ([]Reminder, error) { return nil, ErrNotSupported }
```

## UI Flow

### Key Bindings

| Key | Context | Action |
|-----|---------|--------|
| `T` | Email read view | Create reminder from email |
| `Ctrl+T` | Any view | Open Reminders browser |

### Quick Create Dialog

When pressing `T` in email read view:

```
┌─────────────────────────────────────────────────────┐
│       Create Reminder                               │
├─────────────────────────────────────────────────────┤
│  Title: [Follow up: Q4 Review                   ]   │
│  Due:   [Tomorrow 9:00 AM              ▼]          │
│  List:  [Maily                         ▼]          │
│  Priority: ( ) High  (•) Medium  ( ) Low           │
│                                                     │
│  Notes:                                             │
│  ┌─────────────────────────────────────────────┐   │
│  │ From: john@example.com                      │   │
│  │ Subject: Q4 Review Meeting                  │   │
│  │ ---                                         │   │
│  │ Please review and respond by Friday.        │   │
│  └─────────────────────────────────────────────┘   │
│                                                     │
│  [Create]  [Cancel]                                │
└─────────────────────────────────────────────────────┘
```

### AI-Assisted Quick Create

Press `T` and AI suggests title and due date:

```
┌─────────────────────────────────────────────────────┐
│       Create Reminder                               │
├─────────────────────────────────────────────────────┤
│  AI Suggestion:                                     │
│  "Reply to John about Q4 budget" - Due: Friday     │
│                                                     │
│  [✓ Accept]  [Edit]  [Cancel]                      │
└─────────────────────────────────────────────────────┘
```

### Reminders Browser View

```
┌─ Reminders ─────────────────────────────────────────┐
│ List: Maily (5 tasks)                [/] Search     │
├─────────────────────────────────────────────────────┤
│ ○ Follow up: Q4 Review              Due: Tomorrow   │
│ ○ Reply to Sarah                    Due: Friday     │
│ ○ Send invoice to Acme              Due: Jan 20     │
│ ○ Review contract draft             No due date     │
│ ● Book flight for conference        Done            │
│                                                     │
├─────────────────────────────────────────────────────┤
│ [n]ew  [Enter]complete  [e]dit  [d]elete  [q]uit   │
└─────────────────────────────────────────────────────┘
```

### Due Date Picker

Natural language input with suggestions:

```
┌─ Due Date ──────────────────────────────────────────┐
│ Type or select:  [friday 5pm                    ]   │
├─────────────────────────────────────────────────────┤
│ > Friday, Jan 17, 5:00 PM                          │
│   Tomorrow, 9:00 AM                                │
│   Next Monday, 9:00 AM                             │
│   In 1 week                                        │
│   No due date                                      │
└─────────────────────────────────────────────────────┘
```

## Integration Points

### Files to Create

| File | Purpose |
|------|---------|
| `internal/reminders/reminders.go` | Interface and types |
| `internal/reminders/eventkit_darwin.h` | C header for EventKit |
| `internal/reminders/eventkit_darwin.m` | Objective-C EventKit implementation |
| `internal/reminders/eventkit_darwin.go` | Go wrapper |
| `internal/reminders/stub_other.go` | Non-macOS stub |

### Files to Modify

| File | Changes |
|------|---------|
| `internal/ui/app.go` | Add Reminders states, `T` key handler |
| `internal/ui/commands.go` | Add reminder commands |
| `internal/ui/reminders.go` | New: Reminders browser view |
| `internal/cli/reminders.go` | New: CLI `maily reminders` command |
| `config/config.go` | Add `reminders.list` config |

### App States

```go
const (
    // ... existing states
    stateRemindersBrowser      // Reminders list view
    stateReminderDetail        // Single reminder view
    stateReminderEdit          // Edit reminder
    stateRemindersSearch       // Search results
    stateCreateReminderDialog  // Quick create from email
    stateCreatingReminder      // Spinner while creating
)
```

### CLI Commands

```bash
maily reminders                     # Open Reminders browser TUI
maily reminders list [--list]       # List reminders
maily reminders add "Title" [--due] # Create reminder
maily reminders complete <id>       # Mark as done
maily reminders search <query>      # Search reminders
```

## Reminder Format (from Email)

Default template when creating from email:

**Title**: `"Follow up: {subject}"` or AI-generated

**Notes**:
```
From: sender@example.com
Date: January 14, 2026
Subject: Original subject

---

[First 500 chars of email or key excerpt]
```

## Configuration

```yaml
# ~/.config/maily/config.yml
reminders:
  list: "Maily"              # Default list for email reminders
  default_due: "tomorrow"    # Default due date
  ai_suggestions: true       # Use AI for title/due suggestions
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Non-macOS platform | Show "Not available on this platform" |
| No permission | Prompt: "Grant Maily access in System Settings > Privacy > Reminders" |
| Permission denied | Show error, offer to open System Settings |
| Reminder not found | Show "Reminder not found or was deleted" |
| List not found | Use default list |
| iCloud sync pending | Reminder created locally, syncs automatically |

### Permission Flow

Same as Calendar - uses EventKit authorization:

```go
// First launch or when needed
if !client.Available() {
    granted := client.RequestAccess()
    if !granted {
        // Show permission instructions
    }
}
```

## Testing

### Manual Testing

1. **Create**: Create reminder from email, verify in Reminders.app
2. **Complete**: Check off reminder, verify in Reminders.app
3. **Due dates**: Test various due date inputs
4. **Lists**: Create in different lists
5. **Search**: Search for reminders

### Automated Testing

- Unit tests for Go wrapper logic
- Integration tests require macOS runner
- Mock client for non-macOS CI

## Future Enhancements

1. **Recurring reminders**: Support repeat patterns
2. **Location-based**: Remind at location (home, work)
3. **URL linking**: Deep link back to email in Maily
4. **Subtasks**: Support reminder subtasks (macOS 13+)
5. **Tags**: Support reminder tags (macOS 13+)
6. **Siri integration**: "Hey Siri, remind me about this email"
