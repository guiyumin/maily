# Tauri Reminders Integration

Add emails to macOS Reminders from a Tauri app using direct Obj-C FFI.

## Overview

Create reminders from emails in a Tauri-based email client. Uses direct C FFI to Objective-C EventKit code - same pattern as Calendar integration.

## Architecture

```
src-tauri/
├── src/
│   ├── lib.rs               # Tauri commands registration
│   ├── reminders/
│   │   ├── mod.rs           # Rust FFI bindings + high-level API
│   │   └── eventkit.m       # Objective-C EventKit implementation
│   └── calendar/
│       ├── mod.rs           # (existing) Calendar module
│       └── eventkit.m       # (existing) Calendar Obj-C
├── build.rs                 # Compiles .m files with cc crate
└── Cargo.toml
```

## Setup

### 1. Add Dependencies

```toml
# Cargo.toml
[dependencies]
swift-rs = "1.0"
serde = { version = "1.0", features = ["derive"] }
tauri = { version = "2", features = [] }

[build-dependencies]
swift-rs = { version = "1.0", features = ["build"] }
```

### 2. Build Script

```rust
// build.rs
fn main() {
    // Compile Swift library
    swift_rs::SwiftLinker::new("14.0")
        .with_package("swift-lib", "swift-lib/")
        .link();

    tauri_build::build();
}
```

### 3. Swift Package

```swift
// swift-lib/Package.swift
// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "swift-lib",
    platforms: [.macOS(.v12)],
    products: [
        .library(
            name: "swift-lib",
            type: .static,
            targets: ["SwiftReminders"]
        ),
    ],
    targets: [
        .target(
            name: "SwiftReminders",
            dependencies: [],
            path: "Sources/SwiftReminders"
        ),
    ]
)
```

## Swift Implementation (EventKit)

```swift
// swift-lib/Sources/SwiftReminders/Reminders.swift
import EventKit
import Foundation

// MARK: - Data Types

@frozen
public struct ReminderData {
    public let id: SRString
    public let title: SRString
    public let notes: SRString
    public let dueDate: Double  // Unix timestamp, 0 if none
    public let priority: Int32
    public let completed: Bool
    public let listId: SRString
}

@frozen
public struct ReminderListData {
    public let id: SRString
    public let name: SRString
    public let color: SRString
    public let count: Int32
}

// MARK: - Global Event Store

private let eventStore = EKEventStore()
private var accessGranted = false

// MARK: - Permission

@_cdecl("request_reminders_access")
public func requestRemindersAccess() -> Bool {
    let semaphore = DispatchSemaphore(value: 0)

    if #available(macOS 14.0, *) {
        eventStore.requestFullAccessToReminders { granted, error in
            accessGranted = granted
            semaphore.signal()
        }
    } else {
        eventStore.requestAccess(to: .reminder) { granted, error in
            accessGranted = granted
            semaphore.signal()
        }
    }

    semaphore.wait()
    return accessGranted
}

@_cdecl("check_reminders_access")
public func checkRemindersAccess() -> Bool {
    let status = EKEventStore.authorizationStatus(for: .reminder)
    return status == .authorized
}

// MARK: - Create Reminder

@_cdecl("create_reminder")
public func createReminder(
    title: SRString,
    notes: SRString?,
    dueDate: Double,
    priority: Int32,
    listId: SRString?
) -> SRString? {
    let reminder = EKReminder(eventStore: eventStore)
    reminder.title = title.toString()

    if let notes = notes {
        reminder.notes = notes.toString()
    }

    if dueDate > 0 {
        let date = Date(timeIntervalSince1970: dueDate)
        reminder.dueDateComponents = Calendar.current.dateComponents(
            in: TimeZone.current,
            from: date
        )
    }

    reminder.priority = Int(priority)

    // Find calendar/list
    if let listId = listId,
       let calendar = eventStore.calendar(withIdentifier: listId.toString()) {
        reminder.calendar = calendar
    } else {
        reminder.calendar = eventStore.defaultCalendarForNewReminders()
    }

    do {
        try eventStore.save(reminder, commit: true)
        return SRString(reminder.calendarItemIdentifier)
    } catch {
        print("Failed to create reminder: \(error)")
        return nil
    }
}

// MARK: - Get Reminder

@_cdecl("get_reminder")
public func getReminder(reminderId: SRString) -> ReminderData? {
    guard let item = eventStore.calendarItem(withIdentifier: reminderId.toString()),
          let reminder = item as? EKReminder else {
        return nil
    }

    var dueDate: Double = 0
    if let components = reminder.dueDateComponents,
       let date = Calendar.current.date(from: components) {
        dueDate = date.timeIntervalSince1970
    }

    return ReminderData(
        id: SRString(reminder.calendarItemIdentifier),
        title: SRString(reminder.title ?? ""),
        notes: SRString(reminder.notes ?? ""),
        dueDate: dueDate,
        priority: Int32(reminder.priority),
        completed: reminder.isCompleted,
        listId: SRString(reminder.calendar?.calendarIdentifier ?? "")
    )
}

// MARK: - List Reminders

@_cdecl("list_reminders")
public func listReminders(listId: SRString?, includeCompleted: Bool) -> SRObjectArray {
    var calendars: [EKCalendar]

    if let listId = listId,
       let calendar = eventStore.calendar(withIdentifier: listId.toString()) {
        calendars = [calendar]
    } else {
        calendars = eventStore.calendars(for: .reminder)
    }

    let predicate = eventStore.predicateForReminders(in: calendars)
    var results: [ReminderData] = []

    let semaphore = DispatchSemaphore(value: 0)
    eventStore.fetchReminders(matching: predicate) { reminders in
        guard let reminders = reminders else {
            semaphore.signal()
            return
        }

        for reminder in reminders {
            if !includeCompleted && reminder.isCompleted {
                continue
            }

            var dueDate: Double = 0
            if let components = reminder.dueDateComponents,
               let date = Calendar.current.date(from: components) {
                dueDate = date.timeIntervalSince1970
            }

            results.append(ReminderData(
                id: SRString(reminder.calendarItemIdentifier),
                title: SRString(reminder.title ?? ""),
                notes: SRString(reminder.notes ?? ""),
                dueDate: dueDate,
                priority: Int32(reminder.priority),
                completed: reminder.isCompleted,
                listId: SRString(reminder.calendar?.calendarIdentifier ?? "")
            ))
        }
        semaphore.signal()
    }

    semaphore.wait()
    return SRObjectArray(results)
}

// MARK: - Complete/Uncomplete

@_cdecl("complete_reminder")
public func completeReminder(reminderId: SRString) -> Bool {
    guard let item = eventStore.calendarItem(withIdentifier: reminderId.toString()),
          let reminder = item as? EKReminder else {
        return false
    }

    reminder.isCompleted = true

    do {
        try eventStore.save(reminder, commit: true)
        return true
    } catch {
        return false
    }
}

@_cdecl("uncomplete_reminder")
public func uncompleteReminder(reminderId: SRString) -> Bool {
    guard let item = eventStore.calendarItem(withIdentifier: reminderId.toString()),
          let reminder = item as? EKReminder else {
        return false
    }

    reminder.isCompleted = false

    do {
        try eventStore.save(reminder, commit: true)
        return true
    } catch {
        return false
    }
}

// MARK: - Delete Reminder

@_cdecl("delete_reminder")
public func deleteReminder(reminderId: SRString) -> Bool {
    guard let item = eventStore.calendarItem(withIdentifier: reminderId.toString()),
          let reminder = item as? EKReminder else {
        return false
    }

    do {
        try eventStore.remove(reminder, commit: true)
        return true
    } catch {
        return false
    }
}

// MARK: - List Management

@_cdecl("get_reminder_lists")
public func getReminderLists() -> SRObjectArray {
    let calendars = eventStore.calendars(for: .reminder)
    var results: [ReminderListData] = []

    for calendar in calendars {
        // Count incomplete reminders
        let predicate = eventStore.predicateForReminders(in: [calendar])
        var count: Int32 = 0

        let semaphore = DispatchSemaphore(value: 0)
        eventStore.fetchReminders(matching: predicate) { reminders in
            count = Int32(reminders?.filter { !$0.isCompleted }.count ?? 0)
            semaphore.signal()
        }
        semaphore.wait()

        // Get color as hex
        var colorHex = "#000000"
        if let cgColor = calendar.cgColor {
            let components = cgColor.components ?? [0, 0, 0]
            if components.count >= 3 {
                colorHex = String(format: "#%02X%02X%02X",
                    Int(components[0] * 255),
                    Int(components[1] * 255),
                    Int(components[2] * 255))
            }
        }

        results.append(ReminderListData(
            id: SRString(calendar.calendarIdentifier),
            name: SRString(calendar.title),
            color: SRString(colorHex),
            count: count
        ))
    }

    return SRObjectArray(results)
}
```

## Rust Bindings

```rust
// src-tauri/src/reminders/mod.rs
use swift_rs::{swift, Bool, Int32, SRObject, SRObjectArray, SRString};

pub mod commands;

// Data types matching Swift structs
#[repr(C)]
pub struct ReminderData {
    pub id: SRString,
    pub title: SRString,
    pub notes: SRString,
    pub due_date: f64,
    pub priority: Int32,
    pub completed: Bool,
    pub list_id: SRString,
}

#[repr(C)]
pub struct ReminderListData {
    pub id: SRString,
    pub name: SRString,
    pub color: SRString,
    pub count: Int32,
}

// Swift function bindings
swift!(fn request_reminders_access() -> Bool);
swift!(fn check_reminders_access() -> Bool);
swift!(fn create_reminder(
    title: &SRString,
    notes: Option<&SRString>,
    due_date: f64,
    priority: Int32,
    list_id: Option<&SRString>
) -> Option<SRString>);
swift!(fn get_reminder(reminder_id: &SRString) -> Option<SRObject<ReminderData>>);
swift!(fn list_reminders(list_id: Option<&SRString>, include_completed: Bool) -> SRObjectArray<ReminderData>);
swift!(fn complete_reminder(reminder_id: &SRString) -> Bool);
swift!(fn uncomplete_reminder(reminder_id: &SRString) -> Bool);
swift!(fn delete_reminder(reminder_id: &SRString) -> Bool);
swift!(fn get_reminder_lists() -> SRObjectArray<ReminderListData>);

// High-level Rust API
pub struct RemindersClient;

impl RemindersClient {
    pub fn new() -> Self {
        Self
    }

    pub fn request_access(&self) -> bool {
        unsafe { request_reminders_access() }
    }

    pub fn has_access(&self) -> bool {
        unsafe { check_reminders_access() }
    }

    pub fn create(&self, title: &str, notes: Option<&str>, due_date: Option<f64>, priority: i32, list_id: Option<&str>) -> Option<String> {
        let title = SRString::from(title);
        let notes = notes.map(SRString::from);
        let list_id = list_id.map(SRString::from);

        unsafe {
            create_reminder(
                &title,
                notes.as_ref(),
                due_date.unwrap_or(0.0),
                priority,
                list_id.as_ref(),
            ).map(|s| s.to_string())
        }
    }

    pub fn complete(&self, reminder_id: &str) -> bool {
        let id = SRString::from(reminder_id);
        unsafe { complete_reminder(&id) }
    }

    pub fn delete(&self, reminder_id: &str) -> bool {
        let id = SRString::from(reminder_id);
        unsafe { delete_reminder(&id) }
    }

    pub fn get_lists(&self) -> Vec<ReminderList> {
        let lists = unsafe { get_reminder_lists() };
        lists.iter().map(|l| ReminderList {
            id: l.id.to_string(),
            name: l.name.to_string(),
            color: l.color.to_string(),
            count: l.count,
        }).collect()
    }
}

// Rust-friendly types
#[derive(Debug, Clone, serde::Serialize)]
pub struct Reminder {
    pub id: String,
    pub title: String,
    pub notes: String,
    pub due_date: Option<f64>,
    pub priority: i32,
    pub completed: bool,
    pub list_id: String,
}

#[derive(Debug, Clone, serde::Serialize)]
pub struct ReminderList {
    pub id: String,
    pub name: String,
    pub color: String,
    pub count: i32,
}
```

## Tauri Commands

```rust
// src-tauri/src/reminders/commands.rs
use super::{Reminder, ReminderList, RemindersClient};
use tauri::command;

#[command]
pub async fn reminders_request_access() -> Result<bool, String> {
    let client = RemindersClient::new();
    Ok(client.request_access())
}

#[command]
pub async fn reminders_has_access() -> Result<bool, String> {
    let client = RemindersClient::new();
    Ok(client.has_access())
}

#[command]
pub async fn reminders_create(
    title: String,
    notes: Option<String>,
    due_date: Option<f64>,
    priority: Option<i32>,
    list_id: Option<String>,
) -> Result<String, String> {
    let client = RemindersClient::new();

    client.create(
        &title,
        notes.as_deref(),
        due_date,
        priority.unwrap_or(0),
        list_id.as_deref(),
    ).ok_or_else(|| "Failed to create reminder".to_string())
}

#[command]
pub async fn reminders_complete(reminder_id: String) -> Result<bool, String> {
    let client = RemindersClient::new();
    Ok(client.complete(&reminder_id))
}

#[command]
pub async fn reminders_delete(reminder_id: String) -> Result<bool, String> {
    let client = RemindersClient::new();
    Ok(client.delete(&reminder_id))
}

#[command]
pub async fn reminders_get_lists() -> Result<Vec<ReminderList>, String> {
    let client = RemindersClient::new();
    Ok(client.get_lists())
}
```

## Register Commands

```rust
// src-tauri/src/main.rs
mod reminders;

fn main() {
    tauri::Builder::default()
        .invoke_handler(tauri::generate_handler![
            reminders::commands::reminders_request_access,
            reminders::commands::reminders_has_access,
            reminders::commands::reminders_create,
            reminders::commands::reminders_complete,
            reminders::commands::reminders_delete,
            reminders::commands::reminders_get_lists,
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
```

## Frontend Usage (TypeScript)

```typescript
// src/lib/reminders.ts
import { invoke } from '@tauri-apps/api/core';

export interface Reminder {
  id: string;
  title: string;
  notes: string;
  dueDate?: number;
  priority: number;
  completed: boolean;
  listId: string;
}

export interface ReminderList {
  id: string;
  name: string;
  color: string;
  count: number;
}

export async function requestAccess(): Promise<boolean> {
  return invoke('reminders_request_access');
}

export async function hasAccess(): Promise<boolean> {
  return invoke('reminders_has_access');
}

export async function createReminder(
  title: string,
  options?: {
    notes?: string;
    dueDate?: Date;
    priority?: number;
    listId?: string;
  }
): Promise<string> {
  return invoke('reminders_create', {
    title,
    notes: options?.notes,
    dueDate: options?.dueDate?.getTime() / 1000,
    priority: options?.priority ?? 0,
    listId: options?.listId,
  });
}

export async function completeReminder(reminderId: string): Promise<boolean> {
  return invoke('reminders_complete', { reminderId });
}

export async function deleteReminder(reminderId: string): Promise<boolean> {
  return invoke('reminders_delete', { reminderId });
}

export async function getReminderLists(): Promise<ReminderList[]> {
  return invoke('reminders_get_lists');
}
```

## UI Component (React/Svelte)

```tsx
// Example: Create Reminder from Email
import { createReminder, getReminderLists, requestAccess, hasAccess } from '$lib/reminders';

interface Email {
  subject: string;
  from: string;
  date: string;
  body: string;
}

async function addEmailToReminders(email: Email) {
  // Check/request permission
  if (!await hasAccess()) {
    const granted = await requestAccess();
    if (!granted) {
      alert('Please grant access to Reminders in System Settings');
      return;
    }
  }

  // Create reminder from email
  const title = `Follow up: ${email.subject}`;
  const notes = `From: ${email.from}\nDate: ${email.date}\n\n${email.body.slice(0, 500)}`;

  // Tomorrow at 9am
  const dueDate = new Date();
  dueDate.setDate(dueDate.getDate() + 1);
  dueDate.setHours(9, 0, 0, 0);

  try {
    const reminderId = await createReminder(title, {
      notes,
      dueDate,
      priority: 5, // Medium
    });

    console.log('Created reminder:', reminderId);
    // Show success toast
  } catch (error) {
    console.error('Failed to create reminder:', error);
  }
}
```

## Info.plist

Add to `src-tauri/Info.plist`:

```xml
<key>NSRemindersUsageDescription</key>
<string>Maily needs access to Reminders to create tasks from emails.</string>
```

## Capabilities

Add to `src-tauri/capabilities/default.json`:

```json
{
  "permissions": [
    "core:default",
    "shell:allow-open"
  ]
}
```

## Build Requirements

- macOS 12.0+
- Xcode Command Line Tools
- Swift 5.9+
- Rust 1.70+

## Testing

```bash
# Build and run
cd src-tauri
cargo build

# Test Swift compilation
cd swift-lib
swift build

# Run Tauri app
npm run tauri dev
```

## Error Handling

| Scenario | Behavior |
|----------|----------|
| No permission | Prompt user, open System Settings |
| Swift not found | Build error - install Xcode CLI tools |
| EventKit unavailable | Return error, show message |
| Network offline | Reminder saved locally, syncs when online |

## Platform Support

| Platform | Support |
|----------|---------|
| macOS | Full (EventKit via swift-rs) |
| Windows | Stub (no native Reminders) |
| Linux | Stub (no native Reminders) |

For Windows/Linux, could integrate with:
- Microsoft To Do (Graph API)
- Todoist (REST API)
- Local SQLite fallback
