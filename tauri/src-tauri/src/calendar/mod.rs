// Calendar module - EventKit integration for macOS using objc2
// Pure Rust implementation with objc2 bindings

use serde::{Deserialize, Serialize};

/// Calendar authorization status
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum AuthStatus {
    NotDetermined,
    Restricted,
    Denied,
    Authorized,
}

/// Calendar error types
#[derive(Debug, Clone, Serialize, Deserialize)]
pub enum CalendarError {
    AccessDenied,
    NotFound,
    Failed(String),
    NotSupported,
}

impl std::fmt::Display for CalendarError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            CalendarError::AccessDenied => write!(f, "Calendar access denied"),
            CalendarError::NotFound => write!(f, "Calendar or event not found"),
            CalendarError::Failed(msg) => write!(f, "Calendar operation failed: {}", msg),
            CalendarError::NotSupported => write!(f, "Calendar integration not supported on this platform"),
        }
    }
}

impl std::error::Error for CalendarError {}

/// Calendar representation
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Calendar {
    pub id: String,
    pub title: String,
    pub color: String,
}

/// Calendar event representation
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Event {
    pub id: String,
    pub title: String,
    pub start_time: i64,
    pub end_time: i64,
    pub location: String,
    pub notes: String,
    pub calendar: String,
    pub all_day: bool,
    pub alarm_minutes_before: i32,
}

/// New event request
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct NewEvent {
    pub title: String,
    pub start_time: i64,
    pub end_time: i64,
    #[serde(default)]
    pub location: String,
    #[serde(default)]
    pub notes: String,
    #[serde(default)]
    pub calendar_id: String,
    #[serde(default)]
    pub all_day: bool,
    #[serde(default)]
    pub alarm_minutes_before: i32,
}

// ============ macOS Implementation (objc2) ============

#[cfg(target_os = "macos")]
mod macos {
    use super::*;
    use block2::RcBlock;
    use objc2::rc::Retained;
    use objc2::runtime::Bool;
    use objc2_event_kit::{
        EKAlarm, EKAuthorizationStatus, EKCalendar, EKEntityType, EKEvent, EKEventStore, EKSpan,
    };
    use objc2_foundation::{NSDate, NSError, NSString};
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
        let status = unsafe { EKEventStore::authorizationStatusForEntityType(EKEntityType::Event) };
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

    pub fn request_access() -> Result<(), CalendarError> {
        let status = unsafe { EKEventStore::authorizationStatusForEntityType(EKEntityType::Event) };

        match status {
            EKAuthorizationStatus::FullAccess | EKAuthorizationStatus::WriteOnly => {
                ACCESS_GRANTED.store(true, Ordering::SeqCst);
                return Ok(());
            }
            EKAuthorizationStatus::Denied | EKAuthorizationStatus::Restricted => {
                return Err(CalendarError::AccessDenied);
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
            store.requestFullAccessToEventsWithCompletion(block_ptr);
        }

        match rx.recv() {
            Ok(true) => {
                ACCESS_GRANTED.store(true, Ordering::SeqCst);
                Ok(())
            }
            _ => Err(CalendarError::AccessDenied),
        }
    }

    pub fn list_calendars() -> Result<Vec<Calendar>, CalendarError> {
        if !ACCESS_GRANTED.load(Ordering::SeqCst) {
            return Err(CalendarError::AccessDenied);
        }

        let store = create_event_store();
        let calendars = unsafe { store.calendarsForEntityType(EKEntityType::Event) };

        let mut result = Vec::new();
        let count = calendars.count();
        for i in 0..count {
            let cal: Retained<EKCalendar> = calendars.objectAtIndex(i);
            result.push(Calendar {
                id: retained_to_string(unsafe { &cal.calendarIdentifier() }),
                title: retained_to_string(unsafe { &cal.title() }),
                color: "#808080".to_string(),
            });
        }

        Ok(result)
    }

    pub fn list_events(start_timestamp: i64, end_timestamp: i64) -> Result<Vec<Event>, CalendarError> {
        if !ACCESS_GRANTED.load(Ordering::SeqCst) {
            return Err(CalendarError::AccessDenied);
        }

        let store = create_event_store();

        let start_date = NSDate::dateWithTimeIntervalSince1970(start_timestamp as f64);
        let end_date = NSDate::dateWithTimeIntervalSince1970(end_timestamp as f64);

        let predicate = unsafe {
            store.predicateForEventsWithStartDate_endDate_calendars(&start_date, &end_date, None)
        };

        let events = unsafe { store.eventsMatchingPredicate(&predicate) };

        let mut result = Vec::new();
        let count = events.count();
        for i in 0..count {
            let event: Retained<EKEvent> = events.objectAtIndex(i);
            let alarm_minutes = unsafe {
                event
                    .alarms()
                    .and_then(|alarms| {
                        if alarms.count() > 0 {
                            Some(alarms.objectAtIndex(0))
                        } else {
                            None
                        }
                    })
                    .map(|alarm| (-alarm.relativeOffset() / 60.0) as i32)
                    .unwrap_or(0)
            };

            let start_time = unsafe { event.startDate() }.timeIntervalSince1970() as i64;
            let end_time = unsafe { event.endDate() }.timeIntervalSince1970() as i64;

            result.push(Event {
                id: optional_retained_to_string(unsafe { event.eventIdentifier() }),
                title: retained_to_string(unsafe { &event.title() }),
                start_time,
                end_time,
                location: optional_retained_to_string(unsafe { event.location() }),
                notes: optional_retained_to_string(unsafe { event.notes() }),
                calendar: unsafe { event.calendar() }
                    .map(|c| retained_to_string(unsafe { &c.calendarIdentifier() }))
                    .unwrap_or_default(),
                all_day: unsafe { event.isAllDay() },
                alarm_minutes_before: alarm_minutes,
            });
        }

        Ok(result)
    }

    pub fn create_event(new_event: &NewEvent) -> Result<String, CalendarError> {
        if !ACCESS_GRANTED.load(Ordering::SeqCst) {
            return Err(CalendarError::AccessDenied);
        }

        let store = create_event_store();
        let event = unsafe { EKEvent::eventWithEventStore(&store) };

        unsafe {
            event.setTitle(Some(&ns_string(&new_event.title)));
            event.setStartDate(Some(&NSDate::dateWithTimeIntervalSince1970(
                new_event.start_time as f64,
            )));
            event.setEndDate(Some(&NSDate::dateWithTimeIntervalSince1970(
                new_event.end_time as f64,
            )));
            event.setAllDay(new_event.all_day);

            if !new_event.location.is_empty() {
                event.setLocation(Some(&ns_string(&new_event.location)));
            }

            if !new_event.notes.is_empty() {
                event.setNotes(Some(&ns_string(&new_event.notes)));
            }

            // Set calendar
            let calendar = if !new_event.calendar_id.is_empty() {
                store.calendarWithIdentifier(&ns_string(&new_event.calendar_id))
            } else {
                None
            };
            let calendar = calendar.or_else(|| store.defaultCalendarForNewEvents());
            event.setCalendar(calendar.as_deref());

            // Add alarm if specified
            if new_event.alarm_minutes_before > 0 {
                let alarm =
                    EKAlarm::alarmWithRelativeOffset(-(new_event.alarm_minutes_before as f64 * 60.0));
                event.addAlarm(&alarm);
            }

            // Save
            match store.saveEvent_span_error(&event, EKSpan::ThisEvent) {
                Ok(()) => Ok(optional_retained_to_string(event.eventIdentifier())),
                Err(e) => Err(CalendarError::Failed(e.localizedDescription().to_string())),
            }
        }
    }

    pub fn delete_event(event_id: &str) -> Result<(), CalendarError> {
        if !ACCESS_GRANTED.load(Ordering::SeqCst) {
            return Err(CalendarError::AccessDenied);
        }

        let store = create_event_store();
        let event = unsafe { store.eventWithIdentifier(&ns_string(event_id)) };

        match event {
            Some(event) => unsafe {
                match store.removeEvent_span_error(&event, EKSpan::ThisEvent) {
                    Ok(()) => Ok(()),
                    Err(e) => Err(CalendarError::Failed(e.localizedDescription().to_string())),
                }
            },
            None => Err(CalendarError::NotFound),
        }
    }

    pub fn get_default_calendar() -> Result<String, CalendarError> {
        if !ACCESS_GRANTED.load(Ordering::SeqCst) {
            return Err(CalendarError::AccessDenied);
        }

        let store = create_event_store();
        let calendar = unsafe { store.defaultCalendarForNewEvents() };

        match calendar {
            Some(cal) => Ok(retained_to_string(unsafe { &cal.calendarIdentifier() })),
            None => Err(CalendarError::NotFound),
        }
    }
}

// ============ Non-macOS Stub ============

#[cfg(not(target_os = "macos"))]
mod stub {
    use super::*;

    pub fn get_auth_status() -> AuthStatus {
        AuthStatus::Denied
    }

    pub fn request_access() -> Result<(), CalendarError> {
        Err(CalendarError::NotSupported)
    }

    pub fn list_calendars() -> Result<Vec<Calendar>, CalendarError> {
        Err(CalendarError::NotSupported)
    }

    pub fn list_events(_start: i64, _end: i64) -> Result<Vec<Event>, CalendarError> {
        Err(CalendarError::NotSupported)
    }

    pub fn create_event(_event: &NewEvent) -> Result<String, CalendarError> {
        Err(CalendarError::NotSupported)
    }

    pub fn delete_event(_event_id: &str) -> Result<(), CalendarError> {
        Err(CalendarError::NotSupported)
    }

    pub fn get_default_calendar() -> Result<String, CalendarError> {
        Err(CalendarError::NotSupported)
    }
}

// ============ Public API ============

pub fn get_auth_status() -> AuthStatus {
    #[cfg(target_os = "macos")]
    {
        macos::get_auth_status()
    }
    #[cfg(not(target_os = "macos"))]
    {
        stub::get_auth_status()
    }
}

pub fn request_access() -> Result<(), CalendarError> {
    #[cfg(target_os = "macos")]
    {
        macos::request_access()
    }
    #[cfg(not(target_os = "macos"))]
    {
        stub::request_access()
    }
}

pub fn list_calendars() -> Result<Vec<Calendar>, CalendarError> {
    #[cfg(target_os = "macos")]
    {
        macos::list_calendars()
    }
    #[cfg(not(target_os = "macos"))]
    {
        stub::list_calendars()
    }
}

pub fn list_events(start_timestamp: i64, end_timestamp: i64) -> Result<Vec<Event>, CalendarError> {
    #[cfg(target_os = "macos")]
    {
        macos::list_events(start_timestamp, end_timestamp)
    }
    #[cfg(not(target_os = "macos"))]
    {
        stub::list_events(start_timestamp, end_timestamp)
    }
}

pub fn create_event(event: &NewEvent) -> Result<String, CalendarError> {
    #[cfg(target_os = "macos")]
    {
        macos::create_event(event)
    }
    #[cfg(not(target_os = "macos"))]
    {
        stub::create_event(event)
    }
}

pub fn delete_event(event_id: &str) -> Result<(), CalendarError> {
    #[cfg(target_os = "macos")]
    {
        macos::delete_event(event_id)
    }
    #[cfg(not(target_os = "macos"))]
    {
        stub::delete_event(event_id)
    }
}

pub fn get_default_calendar() -> Result<String, CalendarError> {
    #[cfg(target_os = "macos")]
    {
        macos::get_default_calendar()
    }
    #[cfg(not(target_os = "macos"))]
    {
        stub::get_default_calendar()
    }
}
