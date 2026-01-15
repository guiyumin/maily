// Reminders module - EventKit integration for macOS using objc2
// Pure Rust implementation with objc2 bindings

use serde::{Deserialize, Serialize};

// Priority levels
#[allow(dead_code)]
pub const PRIORITY_NONE: i32 = 0;
#[allow(dead_code)]
pub const PRIORITY_HIGH: i32 = 1;
#[allow(dead_code)]
pub const PRIORITY_MEDIUM: i32 = 5;
#[allow(dead_code)]
pub const PRIORITY_LOW: i32 = 9;

/// Reminder authorization status
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum AuthStatus {
    NotDetermined,
    Restricted,
    Denied,
    Authorized,
}

/// Reminder error types
#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum ReminderError {
    AccessDenied,
    NotFound,
    Failed(String),
    NotSupported,
}

impl std::fmt::Display for ReminderError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            ReminderError::AccessDenied => write!(f, "Reminders access denied"),
            ReminderError::NotFound => write!(f, "Reminder or list not found"),
            ReminderError::Failed(msg) => write!(f, "Reminder operation failed: {}", msg),
            ReminderError::NotSupported => write!(f, "Reminders integration not supported on this platform"),
        }
    }
}

impl std::error::Error for ReminderError {}

/// Reminder list representation
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ReminderList {
    pub id: String,
    pub title: String,
    pub count: i32,
}

/// Reminder representation
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Reminder {
    pub id: String,
    pub title: String,
    pub notes: String,
    pub due_date: Option<i64>,
    pub priority: i32,
    pub completed: bool,
    pub completed_at: Option<i64>,
    pub list_id: String,
    #[serde(default)]
    pub created_at: Option<i64>,
    #[serde(default)]
    pub modified_at: Option<i64>,
}

/// New reminder request
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct NewReminder {
    pub title: String,
    #[serde(default)]
    pub notes: String,
    #[serde(default)]
    pub due_date: Option<i64>,
    #[serde(default)]
    pub priority: i32,
    #[serde(default)]
    pub list_id: String,
}

/// Reminder from email request
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ReminderFromEmail {
    pub email_subject: String,
    pub email_from: String,
    pub email_body: String,
    #[serde(default)]
    pub due_date: Option<i64>,
    #[serde(default)]
    pub priority: i32,
    #[serde(default)]
    pub list_id: String,
}

// ============ macOS Implementation (objc2) ============

#[cfg(target_os = "macos")]
mod macos {
    use super::*;
    use block2::RcBlock;
    use objc2::rc::Retained;
    use objc2::runtime::Bool;
    use objc2_event_kit::{
        EKAuthorizationStatus, EKCalendar, EKEntityType, EKEventStore, EKReminder,
    };
    use objc2_foundation::{NSArray, NSDate, NSError, NSString};
    use std::sync::atomic::{AtomicBool, Ordering};

    static ACCESS_GRANTED: AtomicBool = AtomicBool::new(false);

    fn create_event_store() -> Retained<EKEventStore> {
        unsafe { EKEventStore::new() }
    }

    fn ns_string(s: &str) -> Retained<NSString> {
        NSString::from_str(s)
    }

    fn retained_to_string(ns: &NSString) -> String {
        ns.to_string()
    }

    fn optional_retained_to_string(ns: Option<Retained<NSString>>) -> String {
        ns.map(|s| s.to_string()).unwrap_or_default()
    }

    pub fn get_auth_status() -> AuthStatus {
        let status = unsafe { EKEventStore::authorizationStatusForEntityType(EKEntityType::Reminder) };
        match status {
            EKAuthorizationStatus::NotDetermined => AuthStatus::NotDetermined,
            EKAuthorizationStatus::Restricted => AuthStatus::Restricted,
            EKAuthorizationStatus::Denied => AuthStatus::Denied,
            EKAuthorizationStatus::FullAccess | EKAuthorizationStatus::WriteOnly => {
                ACCESS_GRANTED.store(true, Ordering::SeqCst);
                AuthStatus::Authorized
            }
            _ => AuthStatus::Denied,
        }
    }

    pub fn request_access() -> Result<(), ReminderError> {
        let status = unsafe { EKEventStore::authorizationStatusForEntityType(EKEntityType::Reminder) };

        match status {
            EKAuthorizationStatus::FullAccess | EKAuthorizationStatus::WriteOnly => {
                ACCESS_GRANTED.store(true, Ordering::SeqCst);
                return Ok(());
            }
            EKAuthorizationStatus::Denied | EKAuthorizationStatus::Restricted => {
                return Err(ReminderError::AccessDenied);
            }
            _ => {}
        }

        let store = create_event_store();
        let (tx, rx) = std::sync::mpsc::channel();

        let block = RcBlock::new(move |granted: Bool, _error: *mut NSError| {
            let _ = tx.send(granted.as_bool());
        });

        unsafe {
            let block_ptr = &*block as *const _ as *mut _;
            store.requestFullAccessToRemindersWithCompletion(block_ptr);
        }

        match rx.recv() {
            Ok(true) => {
                ACCESS_GRANTED.store(true, Ordering::SeqCst);
                Ok(())
            }
            _ => Err(ReminderError::AccessDenied),
        }
    }

    pub fn list_lists() -> Result<Vec<ReminderList>, ReminderError> {
        if !ACCESS_GRANTED.load(Ordering::SeqCst) {
            return Err(ReminderError::AccessDenied);
        }

        let store = create_event_store();
        let calendars = unsafe { store.calendarsForEntityType(EKEntityType::Reminder) };

        let mut result = Vec::new();
        let count = calendars.count();
        for i in 0..count {
            let cal: Retained<EKCalendar> = calendars.objectAtIndex(i);
            result.push(ReminderList {
                id: retained_to_string(unsafe { &cal.calendarIdentifier() }),
                title: retained_to_string(unsafe { &cal.title() }),
                count: 0, // Count would require fetching all reminders
            });
        }

        Ok(result)
    }

    pub fn list_reminders(list_id: Option<&str>, include_completed: bool) -> Result<Vec<Reminder>, ReminderError> {
        if !ACCESS_GRANTED.load(Ordering::SeqCst) {
            return Err(ReminderError::AccessDenied);
        }

        let store = create_event_store();

        // Get calendars for the predicate
        let calendars: Option<Retained<NSArray<EKCalendar>>> = if let Some(lid) = list_id {
            unsafe { store.calendarWithIdentifier(&ns_string(lid)) }
                .map(|cal| {
                    NSArray::from_retained_slice(&[cal])
                })
        } else {
            None
        };

        let predicate = unsafe {
            store.predicateForRemindersInCalendars(calendars.as_deref())
        };

        // Fetch reminders synchronously using channel pattern
        let (tx, rx) = std::sync::mpsc::channel();

        let block = RcBlock::new(move |reminders: *mut NSArray<EKReminder>| {
            let _ = tx.send(if reminders.is_null() {
                None
            } else {
                Some(unsafe { Retained::retain(reminders).unwrap() })
            });
        });

        unsafe {
            store.fetchRemindersMatchingPredicate_completion(&predicate, &*block);
        }

        let reminders = rx.recv()
            .map_err(|_| ReminderError::Failed("Failed to fetch reminders".to_string()))?
            .ok_or_else(|| ReminderError::Failed("No reminders returned".to_string()))?;

        let mut result = Vec::new();
        let count = reminders.count();
        for i in 0..count {
            let reminder: Retained<EKReminder> = reminders.objectAtIndex(i);
            let is_completed = unsafe { reminder.isCompleted() };

            // Skip completed reminders if not requested
            if !include_completed && is_completed {
                continue;
            }

            let due_date = unsafe { reminder.dueDateComponents() }
                .and_then(|components| components.date())
                .map(|d| d.timeIntervalSince1970() as i64);

            let completed_at = if is_completed {
                unsafe { reminder.completionDate() }
                    .map(|d| d.timeIntervalSince1970() as i64)
            } else {
                None
            };

            let created_at = unsafe { reminder.creationDate() }
                .map(|d| d.timeIntervalSince1970() as i64);

            let modified_at = unsafe { reminder.lastModifiedDate() }
                .map(|d| d.timeIntervalSince1970() as i64);

            result.push(Reminder {
                id: retained_to_string(unsafe { &reminder.calendarItemIdentifier() }),
                title: retained_to_string(unsafe { &reminder.title() }),
                notes: optional_retained_to_string(unsafe { reminder.notes() }),
                due_date,
                priority: unsafe { reminder.priority() } as i32,
                completed: is_completed,
                completed_at,
                list_id: unsafe { reminder.calendar() }
                    .map(|c| retained_to_string(unsafe { &c.calendarIdentifier() }))
                    .unwrap_or_default(),
                created_at,
                modified_at,
            });
        }

        Ok(result)
    }

    pub fn get_reminder(reminder_id: &str) -> Result<Reminder, ReminderError> {
        if !ACCESS_GRANTED.load(Ordering::SeqCst) {
            return Err(ReminderError::AccessDenied);
        }

        let store = create_event_store();
        let item = unsafe { store.calendarItemWithIdentifier(&ns_string(reminder_id)) };

        match item {
            Some(item) => {
                // Cast to EKReminder
                let reminder: Retained<EKReminder> = unsafe {
                    Retained::cast_unchecked(item)
                };

                let is_completed = unsafe { reminder.isCompleted() };

                let due_date = unsafe { reminder.dueDateComponents() }
                    .and_then(|components| components.date())
                    .map(|d| d.timeIntervalSince1970() as i64);

                let completed_at = if is_completed {
                    unsafe { reminder.completionDate() }
                        .map(|d| d.timeIntervalSince1970() as i64)
                } else {
                    None
                };

                let created_at = unsafe { reminder.creationDate() }
                    .map(|d| d.timeIntervalSince1970() as i64);

                let modified_at = unsafe { reminder.lastModifiedDate() }
                    .map(|d| d.timeIntervalSince1970() as i64);

                Ok(Reminder {
                    id: retained_to_string(unsafe { &reminder.calendarItemIdentifier() }),
                    title: retained_to_string(unsafe { &reminder.title() }),
                    notes: optional_retained_to_string(unsafe { reminder.notes() }),
                    due_date,
                    priority: unsafe { reminder.priority() } as i32,
                    completed: is_completed,
                    completed_at,
                    list_id: unsafe { reminder.calendar() }
                        .map(|c| retained_to_string(unsafe { &c.calendarIdentifier() }))
                        .unwrap_or_default(),
                    created_at,
                    modified_at,
                })
            }
            None => Err(ReminderError::NotFound),
        }
    }

    pub fn create_reminder(new_reminder: &NewReminder) -> Result<String, ReminderError> {
        if !ACCESS_GRANTED.load(Ordering::SeqCst) {
            return Err(ReminderError::AccessDenied);
        }

        let store = create_event_store();
        let reminder = unsafe { EKReminder::reminderWithEventStore(&store) };

        unsafe {
            reminder.setTitle(Some(&ns_string(&new_reminder.title)));

            if !new_reminder.notes.is_empty() {
                reminder.setNotes(Some(&ns_string(&new_reminder.notes)));
            }

            // Set calendar
            let calendar = if !new_reminder.list_id.is_empty() {
                store.calendarWithIdentifier(&ns_string(&new_reminder.list_id))
            } else {
                None
            };
            let calendar = calendar.or_else(|| store.defaultCalendarForNewReminders());
            reminder.setCalendar(calendar.as_deref());

            // Set priority
            if new_reminder.priority > 0 {
                reminder.setPriority(new_reminder.priority as usize);
            }

            // Set due date using NSDateComponents
            if let Some(timestamp) = new_reminder.due_date {
                let date = NSDate::dateWithTimeIntervalSince1970(timestamp as f64);
                let calendar_obj = objc2_foundation::NSCalendar::currentCalendar();
                let units = objc2_foundation::NSCalendarUnit::Year
                    | objc2_foundation::NSCalendarUnit::Month
                    | objc2_foundation::NSCalendarUnit::Day
                    | objc2_foundation::NSCalendarUnit::Hour
                    | objc2_foundation::NSCalendarUnit::Minute;
                let components = calendar_obj.components_fromDate(units, &date);
                reminder.setDueDateComponents(Some(&components));
            }

            // Save
            match store.saveReminder_commit_error(&reminder, true) {
                Ok(()) => Ok(retained_to_string(&reminder.calendarItemIdentifier())),
                Err(e) => Err(ReminderError::Failed(e.localizedDescription().to_string())),
            }
        }
    }

    pub fn create_reminder_from_email(req: &ReminderFromEmail) -> Result<String, ReminderError> {
        let title = format!("Follow up: {}", req.email_subject);
        let notes = format!(
            "From: {}\n\n---\n\n{}",
            req.email_from,
            if req.email_body.len() > 500 {
                format!("{}...", &req.email_body[..500])
            } else {
                req.email_body.clone()
            }
        );

        let reminder = NewReminder {
            title,
            notes,
            due_date: req.due_date,
            priority: req.priority,
            list_id: req.list_id.clone(),
        };

        create_reminder(&reminder)
    }

    pub fn update_reminder(
        reminder_id: &str,
        title: Option<&str>,
        notes: Option<&str>,
        due_date: Option<i64>,
        clear_due_date: bool,
        priority: i32,
    ) -> Result<(), ReminderError> {
        if !ACCESS_GRANTED.load(Ordering::SeqCst) {
            return Err(ReminderError::AccessDenied);
        }

        let store = create_event_store();
        let item = unsafe { store.calendarItemWithIdentifier(&ns_string(reminder_id)) };

        match item {
            Some(item) => {
                let reminder: Retained<EKReminder> = unsafe { Retained::cast_unchecked(item) };

                unsafe {
                    if let Some(t) = title {
                        reminder.setTitle(Some(&ns_string(t)));
                    }

                    if let Some(n) = notes {
                        reminder.setNotes(Some(&ns_string(n)));
                    }

                    if priority > 0 {
                        reminder.setPriority(priority as usize);
                    }

                    if clear_due_date {
                        reminder.setDueDateComponents(None);
                    } else if let Some(timestamp) = due_date {
                        let date = NSDate::dateWithTimeIntervalSince1970(timestamp as f64);
                        let calendar_obj = objc2_foundation::NSCalendar::currentCalendar();
                        let units = objc2_foundation::NSCalendarUnit::Year
                            | objc2_foundation::NSCalendarUnit::Month
                            | objc2_foundation::NSCalendarUnit::Day
                            | objc2_foundation::NSCalendarUnit::Hour
                            | objc2_foundation::NSCalendarUnit::Minute;
                        let components = calendar_obj.components_fromDate(units, &date);
                        reminder.setDueDateComponents(Some(&components));
                    }

                    match store.saveReminder_commit_error(&reminder, true) {
                        Ok(()) => Ok(()),
                        Err(e) => Err(ReminderError::Failed(e.localizedDescription().to_string())),
                    }
                }
            }
            None => Err(ReminderError::NotFound),
        }
    }

    pub fn delete_reminder(reminder_id: &str) -> Result<(), ReminderError> {
        if !ACCESS_GRANTED.load(Ordering::SeqCst) {
            return Err(ReminderError::AccessDenied);
        }

        let store = create_event_store();
        let item = unsafe { store.calendarItemWithIdentifier(&ns_string(reminder_id)) };

        match item {
            Some(item) => {
                let reminder: Retained<EKReminder> = unsafe { Retained::cast_unchecked(item) };
                unsafe {
                    match store.removeReminder_commit_error(&reminder, true) {
                        Ok(()) => Ok(()),
                        Err(e) => Err(ReminderError::Failed(e.localizedDescription().to_string())),
                    }
                }
            }
            None => Err(ReminderError::NotFound),
        }
    }

    pub fn complete_reminder(reminder_id: &str) -> Result<(), ReminderError> {
        if !ACCESS_GRANTED.load(Ordering::SeqCst) {
            return Err(ReminderError::AccessDenied);
        }

        let store = create_event_store();
        let item = unsafe { store.calendarItemWithIdentifier(&ns_string(reminder_id)) };

        match item {
            Some(item) => {
                let reminder: Retained<EKReminder> = unsafe { Retained::cast_unchecked(item) };
                unsafe {
                    reminder.setCompleted(true);
                    match store.saveReminder_commit_error(&reminder, true) {
                        Ok(()) => Ok(()),
                        Err(e) => Err(ReminderError::Failed(e.localizedDescription().to_string())),
                    }
                }
            }
            None => Err(ReminderError::NotFound),
        }
    }

    pub fn uncomplete_reminder(reminder_id: &str) -> Result<(), ReminderError> {
        if !ACCESS_GRANTED.load(Ordering::SeqCst) {
            return Err(ReminderError::AccessDenied);
        }

        let store = create_event_store();
        let item = unsafe { store.calendarItemWithIdentifier(&ns_string(reminder_id)) };

        match item {
            Some(item) => {
                let reminder: Retained<EKReminder> = unsafe { Retained::cast_unchecked(item) };
                unsafe {
                    reminder.setCompleted(false);
                    reminder.setCompletionDate(None);
                    match store.saveReminder_commit_error(&reminder, true) {
                        Ok(()) => Ok(()),
                        Err(e) => Err(ReminderError::Failed(e.localizedDescription().to_string())),
                    }
                }
            }
            None => Err(ReminderError::NotFound),
        }
    }

    pub fn get_default_list() -> Result<String, ReminderError> {
        if !ACCESS_GRANTED.load(Ordering::SeqCst) {
            return Err(ReminderError::AccessDenied);
        }

        let store = create_event_store();
        let calendar = unsafe { store.defaultCalendarForNewReminders() };

        match calendar {
            Some(cal) => Ok(retained_to_string(unsafe { &cal.calendarIdentifier() })),
            None => Err(ReminderError::NotFound),
        }
    }

    pub fn search_reminders(query: &str, list_id: Option<&str>) -> Result<Vec<Reminder>, ReminderError> {
        // Get all reminders and filter by query
        let all_reminders = list_reminders(list_id, true)?;
        let query_lower = query.to_lowercase();

        Ok(all_reminders
            .into_iter()
            .filter(|r| {
                r.title.to_lowercase().contains(&query_lower)
                    || r.notes.to_lowercase().contains(&query_lower)
            })
            .collect())
    }
}

// ============ Non-macOS Stub ============

#[cfg(not(target_os = "macos"))]
mod stub {
    use super::*;

    pub fn get_auth_status() -> AuthStatus {
        AuthStatus::Denied
    }

    pub fn request_access() -> Result<(), ReminderError> {
        Err(ReminderError::NotSupported)
    }

    pub fn list_lists() -> Result<Vec<ReminderList>, ReminderError> {
        Err(ReminderError::NotSupported)
    }

    pub fn list_reminders(_list_id: Option<&str>, _include_completed: bool) -> Result<Vec<Reminder>, ReminderError> {
        Err(ReminderError::NotSupported)
    }

    pub fn get_reminder(_reminder_id: &str) -> Result<Reminder, ReminderError> {
        Err(ReminderError::NotSupported)
    }

    pub fn create_reminder(_reminder: &NewReminder) -> Result<String, ReminderError> {
        Err(ReminderError::NotSupported)
    }

    pub fn create_reminder_from_email(_req: &ReminderFromEmail) -> Result<String, ReminderError> {
        Err(ReminderError::NotSupported)
    }

    pub fn update_reminder(
        _reminder_id: &str,
        _title: Option<&str>,
        _notes: Option<&str>,
        _due_date: Option<i64>,
        _clear_due_date: bool,
        _priority: i32,
    ) -> Result<(), ReminderError> {
        Err(ReminderError::NotSupported)
    }

    pub fn delete_reminder(_reminder_id: &str) -> Result<(), ReminderError> {
        Err(ReminderError::NotSupported)
    }

    pub fn complete_reminder(_reminder_id: &str) -> Result<(), ReminderError> {
        Err(ReminderError::NotSupported)
    }

    pub fn uncomplete_reminder(_reminder_id: &str) -> Result<(), ReminderError> {
        Err(ReminderError::NotSupported)
    }

    pub fn get_default_list() -> Result<String, ReminderError> {
        Err(ReminderError::NotSupported)
    }

    pub fn search_reminders(_query: &str, _list_id: Option<&str>) -> Result<Vec<Reminder>, ReminderError> {
        Err(ReminderError::NotSupported)
    }
}

// ============ Public API ============

pub fn get_auth_status() -> AuthStatus {
    #[cfg(target_os = "macos")]
    { macos::get_auth_status() }
    #[cfg(not(target_os = "macos"))]
    { stub::get_auth_status() }
}

pub fn request_access() -> Result<(), ReminderError> {
    #[cfg(target_os = "macos")]
    { macos::request_access() }
    #[cfg(not(target_os = "macos"))]
    { stub::request_access() }
}

pub fn list_lists() -> Result<Vec<ReminderList>, ReminderError> {
    #[cfg(target_os = "macos")]
    { macos::list_lists() }
    #[cfg(not(target_os = "macos"))]
    { stub::list_lists() }
}

pub fn list_reminders(list_id: Option<&str>, include_completed: bool) -> Result<Vec<Reminder>, ReminderError> {
    #[cfg(target_os = "macos")]
    { macos::list_reminders(list_id, include_completed) }
    #[cfg(not(target_os = "macos"))]
    { stub::list_reminders(list_id, include_completed) }
}

pub fn get_reminder(reminder_id: &str) -> Result<Reminder, ReminderError> {
    #[cfg(target_os = "macos")]
    { macos::get_reminder(reminder_id) }
    #[cfg(not(target_os = "macos"))]
    { stub::get_reminder(reminder_id) }
}

pub fn create_reminder(reminder: &NewReminder) -> Result<String, ReminderError> {
    #[cfg(target_os = "macos")]
    { macos::create_reminder(reminder) }
    #[cfg(not(target_os = "macos"))]
    { stub::create_reminder(reminder) }
}

pub fn create_reminder_from_email(req: &ReminderFromEmail) -> Result<String, ReminderError> {
    #[cfg(target_os = "macos")]
    { macos::create_reminder_from_email(req) }
    #[cfg(not(target_os = "macos"))]
    { stub::create_reminder_from_email(req) }
}

pub fn update_reminder(
    reminder_id: &str,
    title: Option<&str>,
    notes: Option<&str>,
    due_date: Option<i64>,
    clear_due_date: bool,
    priority: i32,
) -> Result<(), ReminderError> {
    #[cfg(target_os = "macos")]
    { macos::update_reminder(reminder_id, title, notes, due_date, clear_due_date, priority) }
    #[cfg(not(target_os = "macos"))]
    { stub::update_reminder(reminder_id, title, notes, due_date, clear_due_date, priority) }
}

pub fn delete_reminder(reminder_id: &str) -> Result<(), ReminderError> {
    #[cfg(target_os = "macos")]
    { macos::delete_reminder(reminder_id) }
    #[cfg(not(target_os = "macos"))]
    { stub::delete_reminder(reminder_id) }
}

pub fn complete_reminder(reminder_id: &str) -> Result<(), ReminderError> {
    #[cfg(target_os = "macos")]
    { macos::complete_reminder(reminder_id) }
    #[cfg(not(target_os = "macos"))]
    { stub::complete_reminder(reminder_id) }
}

pub fn uncomplete_reminder(reminder_id: &str) -> Result<(), ReminderError> {
    #[cfg(target_os = "macos")]
    { macos::uncomplete_reminder(reminder_id) }
    #[cfg(not(target_os = "macos"))]
    { stub::uncomplete_reminder(reminder_id) }
}

pub fn get_default_list() -> Result<String, ReminderError> {
    #[cfg(target_os = "macos")]
    { macos::get_default_list() }
    #[cfg(not(target_os = "macos"))]
    { stub::get_default_list() }
}

pub fn search_reminders(query: &str, list_id: Option<&str>) -> Result<Vec<Reminder>, ReminderError> {
    #[cfg(target_os = "macos")]
    { macos::search_reminders(query, list_id) }
    #[cfg(not(target_os = "macos"))]
    { stub::search_reminders(query, list_id) }
}
