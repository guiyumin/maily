// Calendar module - EventKit integration for macOS
// Provides native calendar access via Rust FFI to Objective-C

use serde::{Deserialize, Serialize};

#[cfg(target_os = "macos")]
mod ffi {
    use std::os::raw::{c_char, c_double, c_int};

    extern "C" {
        pub fn ek_get_auth_status() -> c_int;
        pub fn ek_request_access() -> c_int;
        pub fn ek_list_calendars(result_code: *mut c_int) -> *mut c_char;
        pub fn ek_list_events(
            start_timestamp: c_double,
            end_timestamp: c_double,
            result_code: *mut c_int,
        ) -> *mut c_char;
        pub fn ek_create_event(
            title: *const c_char,
            start_timestamp: c_double,
            end_timestamp: c_double,
            location: *const c_char,
            notes: *const c_char,
            calendar_id: *const c_char,
            all_day: c_int,
            alarm_minutes_before: c_int,
            result_code: *mut c_int,
        ) -> *mut c_char;
        pub fn ek_delete_event(event_id: *const c_char) -> c_int;
        pub fn ek_get_default_calendar(result_code: *mut c_int) -> *mut c_char;
        pub fn ek_free_string(str: *mut c_char);
    }
}

// Result codes from Objective-C
const EK_SUCCESS: i32 = 0;
const EK_ERROR_ACCESS_DENIED: i32 = 1;
const EK_ERROR_NOT_FOUND: i32 = 2;
const EK_ERROR_FAILED: i32 = 3;

// Authorization status codes
const EK_AUTH_NOT_DETERMINED: i32 = 0;
const EK_AUTH_RESTRICTED: i32 = 1;
const EK_AUTH_DENIED: i32 = 2;
const EK_AUTH_AUTHORIZED: i32 = 3;

/// Calendar authorization status
#[derive(Debug, Clone, Copy, PartialEq, Eq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum AuthStatus {
    NotDetermined,
    Restricted,
    Denied,
    Authorized,
}

impl From<i32> for AuthStatus {
    fn from(code: i32) -> Self {
        match code {
            EK_AUTH_NOT_DETERMINED => AuthStatus::NotDetermined,
            EK_AUTH_RESTRICTED => AuthStatus::Restricted,
            EK_AUTH_DENIED => AuthStatus::Denied,
            EK_AUTH_AUTHORIZED => AuthStatus::Authorized,
            _ => AuthStatus::Denied,
        }
    }
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

fn result_code_to_error(code: i32) -> CalendarError {
    match code {
        EK_ERROR_ACCESS_DENIED => CalendarError::AccessDenied,
        EK_ERROR_NOT_FOUND => CalendarError::NotFound,
        _ => CalendarError::Failed("Unknown error".to_string()),
    }
}

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
    pub start_time: i64,  // Unix timestamp
    pub end_time: i64,    // Unix timestamp
    pub location: String,
    pub notes: String,
    pub calendar: String,
    pub all_day: bool,
    pub alarm_minutes_before: i32,
}

/// New event request (for creating events)
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

// ============ macOS Implementation ============

#[cfg(target_os = "macos")]
mod macos {
    use super::*;
    use std::ffi::{CStr, CString};
    use std::os::raw::c_int;

    /// Get current authorization status without prompting
    pub fn get_auth_status() -> AuthStatus {
        let status = unsafe { ffi::ek_get_auth_status() };
        AuthStatus::from(status)
    }

    /// Request calendar access (will prompt user if needed)
    pub fn request_access() -> Result<(), CalendarError> {
        let result = unsafe { ffi::ek_request_access() };
        if result == EK_SUCCESS {
            Ok(())
        } else {
            Err(result_code_to_error(result))
        }
    }

    /// List all available calendars
    pub fn list_calendars() -> Result<Vec<Calendar>, CalendarError> {
        let mut result_code: c_int = 0;
        let json_ptr = unsafe { ffi::ek_list_calendars(&mut result_code) };

        if result_code != EK_SUCCESS || json_ptr.is_null() {
            return Err(result_code_to_error(result_code));
        }

        let json_str = unsafe {
            let cstr = CStr::from_ptr(json_ptr);
            let s = cstr.to_string_lossy().into_owned();
            ffi::ek_free_string(json_ptr);
            s
        };

        serde_json::from_str(&json_str)
            .map_err(|e| CalendarError::Failed(format!("Failed to parse calendars: {}", e)))
    }

    /// List events in a date range
    pub fn list_events(start_timestamp: i64, end_timestamp: i64) -> Result<Vec<Event>, CalendarError> {
        let mut result_code: c_int = 0;
        let json_ptr = unsafe {
            ffi::ek_list_events(
                start_timestamp as f64,
                end_timestamp as f64,
                &mut result_code,
            )
        };

        if result_code != EK_SUCCESS || json_ptr.is_null() {
            return Err(result_code_to_error(result_code));
        }

        let json_str = unsafe {
            let cstr = CStr::from_ptr(json_ptr);
            let s = cstr.to_string_lossy().into_owned();
            ffi::ek_free_string(json_ptr);
            s
        };

        serde_json::from_str(&json_str)
            .map_err(|e| CalendarError::Failed(format!("Failed to parse events: {}", e)))
    }

    /// Create a new event
    pub fn create_event(event: &NewEvent) -> Result<String, CalendarError> {
        let title = CString::new(event.title.as_str()).unwrap_or_default();
        let location = CString::new(event.location.as_str()).unwrap_or_default();
        let notes = CString::new(event.notes.as_str()).unwrap_or_default();
        let calendar_id = CString::new(event.calendar_id.as_str()).unwrap_or_default();

        let mut result_code: c_int = 0;
        let event_id_ptr = unsafe {
            ffi::ek_create_event(
                title.as_ptr(),
                event.start_time as f64,
                event.end_time as f64,
                location.as_ptr(),
                notes.as_ptr(),
                calendar_id.as_ptr(),
                if event.all_day { 1 } else { 0 },
                event.alarm_minutes_before,
                &mut result_code,
            )
        };

        if result_code != EK_SUCCESS || event_id_ptr.is_null() {
            return Err(result_code_to_error(result_code));
        }

        let event_id = unsafe {
            let cstr = CStr::from_ptr(event_id_ptr);
            let s = cstr.to_string_lossy().into_owned();
            ffi::ek_free_string(event_id_ptr);
            s
        };

        Ok(event_id)
    }

    /// Delete an event by ID
    pub fn delete_event(event_id: &str) -> Result<(), CalendarError> {
        let event_id_cstr = CString::new(event_id).unwrap_or_default();
        let result = unsafe { ffi::ek_delete_event(event_id_cstr.as_ptr()) };

        if result == EK_SUCCESS {
            Ok(())
        } else {
            Err(result_code_to_error(result))
        }
    }

    /// Get the default calendar ID
    pub fn get_default_calendar() -> Result<String, CalendarError> {
        let mut result_code: c_int = 0;
        let cal_id_ptr = unsafe { ffi::ek_get_default_calendar(&mut result_code) };

        if result_code != EK_SUCCESS || cal_id_ptr.is_null() {
            return Err(result_code_to_error(result_code));
        }

        let cal_id = unsafe {
            let cstr = CStr::from_ptr(cal_id_ptr);
            let s = cstr.to_string_lossy().into_owned();
            ffi::ek_free_string(cal_id_ptr);
            s
        };

        Ok(cal_id)
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

/// Get current calendar authorization status
pub fn get_auth_status() -> AuthStatus {
    #[cfg(target_os = "macos")]
    { macos::get_auth_status() }
    #[cfg(not(target_os = "macos"))]
    { stub::get_auth_status() }
}

/// Request calendar access permission
pub fn request_access() -> Result<(), CalendarError> {
    #[cfg(target_os = "macos")]
    { macos::request_access() }
    #[cfg(not(target_os = "macos"))]
    { stub::request_access() }
}

/// List all available calendars
pub fn list_calendars() -> Result<Vec<Calendar>, CalendarError> {
    #[cfg(target_os = "macos")]
    { macos::list_calendars() }
    #[cfg(not(target_os = "macos"))]
    { stub::list_calendars() }
}

/// List events in a date range (Unix timestamps)
pub fn list_events(start_timestamp: i64, end_timestamp: i64) -> Result<Vec<Event>, CalendarError> {
    #[cfg(target_os = "macos")]
    { macos::list_events(start_timestamp, end_timestamp) }
    #[cfg(not(target_os = "macos"))]
    { stub::list_events(start_timestamp, end_timestamp) }
}

/// Create a new calendar event
pub fn create_event(event: &NewEvent) -> Result<String, CalendarError> {
    #[cfg(target_os = "macos")]
    { macos::create_event(event) }
    #[cfg(not(target_os = "macos"))]
    { stub::create_event(event) }
}

/// Delete an event by ID
pub fn delete_event(event_id: &str) -> Result<(), CalendarError> {
    #[cfg(target_os = "macos")]
    { macos::delete_event(event_id) }
    #[cfg(not(target_os = "macos"))]
    { stub::delete_event(event_id) }
}

/// Get the default calendar ID
pub fn get_default_calendar() -> Result<String, CalendarError> {
    #[cfg(target_os = "macos")]
    { macos::get_default_calendar() }
    #[cfg(not(target_os = "macos"))]
    { stub::get_default_calendar() }
}
