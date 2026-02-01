mod ai;
mod calendar;
mod config;
mod imap_queue;
mod mail;
mod reminders;
mod smtp;

use tauri::{
    Manager, RunEvent, WebviewWindowBuilder, WindowEvent,
    tray::TrayIconBuilder,
    image::Image,
};
use ai::{
    init_summaries_table, get_cached_summary, delete_summary, list_available_providers,
    cli_complete as do_cli_complete, save_summary_from_frontend,
    CompletionRequest, CompletionResponse, EmailSummary,
};
use config::{get_config as load_config, save_config as store_config, send_telegram_test, AIProvider, Config};
use imap_queue::{init as init_imap_queue, queue_mark_read, queue_move_to_trash, queue_sync};
use mail::{
    delete_email_from_cache, get_accounts, get_email as fetch_email, get_emails,
    get_email_with_body as fetch_email_with_body,
    list_emails_paginated, get_emails_count_since_days, init_db, get_initial_state,
    sync_emails as do_sync_emails, update_email_read_status, update_email_cache_only,
    get_all_unread_counts as fetch_all_unread_counts,
    get_unread_count as fetch_unread_count,
    get_mailbox_unread_counts as fetch_mailbox_unread_counts,
    save_draft as mail_save_draft, get_draft as mail_get_draft,
    list_drafts as mail_list_drafts, delete_draft as mail_delete_draft, log_op,
    add_account as mail_add_account, remove_account as mail_remove_account,
    update_account as mail_update_account, test_account_credentials,
    upload_avatar as mail_upload_avatar, delete_avatar as mail_delete_avatar,
    get_avatar_path as mail_get_avatar_path,
    Account, Email, ListEmailsResult, SyncResult, InitialState, Draft,
};
use smtp::{send_email as smtp_send, save_draft_to_imap, ComposeEmail, SendResult, AttachmentInfo};
use calendar::{
    AuthStatus as CalendarAuthStatus, Calendar as CalendarInfo, Event as CalendarEvent,
    NewEvent, get_auth_status as cal_auth_status, request_access as cal_request_access,
    list_calendars as cal_list_calendars, list_events as cal_list_events,
    create_event as cal_create_event, delete_event as cal_delete_event,
    get_default_calendar as cal_default_calendar, search_events as cal_search_events,
};
use reminders::{
    AuthStatus as ReminderAuthStatus, Reminder, ReminderList, NewReminder, ReminderFromEmail,
    get_auth_status as rem_auth_status, request_access as rem_request_access,
    list_lists as rem_list_lists, list_reminders as rem_list_reminders,
    get_reminder as rem_get, create_reminder as rem_create,
    create_reminder_from_email as rem_create_from_email, update_reminder as rem_update,
    delete_reminder as rem_delete, complete_reminder as rem_complete,
    uncomplete_reminder as rem_uncomplete, get_default_list as rem_default_list,
    search_reminders as rem_search,
};

#[tauri::command]
fn list_full_accounts() -> Result<Vec<Account>, String> {
    get_accounts().map_err(|e| e.to_string())
}

#[tauri::command]
fn add_account(account: Account) -> Result<Vec<Account>, String> {
    mail_add_account(account).map_err(|e| e.to_string())
}

#[tauri::command]
fn remove_account(name: String) -> Result<Vec<Account>, String> {
    mail_remove_account(&name).map_err(|e| e.to_string())
}

#[tauri::command]
fn update_account(name: String, account: Account) -> Result<Vec<Account>, String> {
    mail_update_account(&name, account).map_err(|e| e.to_string())
}

#[tauri::command]
fn test_account(email: String, password: String, imap_host: String, imap_port: u16) -> Result<(), String> {
    test_account_credentials(&email, &password, &imap_host, imap_port).map_err(|e| e.to_string())
}

/// Upload avatar for an account from a file path.
/// Updates the account's avatar field and returns updated accounts list.
#[tauri::command]
fn upload_account_avatar(
    account_name: String,
    file_path: String,
) -> Result<Vec<Account>, String> {
    use std::path::Path;

    let path = Path::new(&file_path);
    let extension = path
        .extension()
        .and_then(|e| e.to_str())
        .unwrap_or("png")
        .to_lowercase();

    // Read the file
    let image_data = std::fs::read(&file_path)
        .map_err(|e| format!("Failed to read image file: {}", e))?;

    // Upload the avatar file
    let filename = mail_upload_avatar(&account_name, &image_data, &extension)
        .map_err(|e| e.to_string())?;

    // Update the account with the avatar filename
    let mut accounts = get_accounts().map_err(|e| e.to_string())?;
    if let Some(account) = accounts.iter_mut().find(|a| a.name == account_name) {
        // Delete old avatar if exists
        if let Some(old_avatar) = &account.avatar {
            let _ = mail_delete_avatar(old_avatar);
        }
        account.avatar = Some(filename);
    }

    // Save accounts
    mail_update_account(&account_name, accounts.iter().find(|a| a.name == account_name).cloned().ok_or("Account not found")?)
        .map_err(|e| e.to_string())
}

/// Delete avatar for an account
#[tauri::command]
fn delete_account_avatar(account_name: String) -> Result<Vec<Account>, String> {
    let mut accounts = get_accounts().map_err(|e| e.to_string())?;
    if let Some(account) = accounts.iter_mut().find(|a| a.name == account_name) {
        if let Some(avatar) = &account.avatar {
            mail_delete_avatar(avatar).map_err(|e| e.to_string())?;
        }
        let mut updated = account.clone();
        updated.avatar = None;
        return mail_update_account(&account_name, updated).map_err(|e| e.to_string());
    }
    Err("Account not found".to_string())
}

/// Get avatar URLs for all accounts (email -> base64 data URL)
#[tauri::command]
fn get_account_avatar_urls() -> Result<std::collections::HashMap<String, String>, String> {
    use base64::{Engine, engine::general_purpose::STANDARD};

    let accounts = get_accounts().map_err(|e| e.to_string())?;
    let mut urls = std::collections::HashMap::new();

    for account in accounts {
        if let Some(avatar) = &account.avatar {
            if let Some(path) = mail_get_avatar_path(avatar) {
                if let Ok(data) = std::fs::read(&path) {
                    let ext = path.extension()
                        .and_then(|e| e.to_str())
                        .unwrap_or("png")
                        .to_lowercase();
                    let mime = match ext.as_str() {
                        "jpg" | "jpeg" => "image/jpeg",
                        "png" => "image/png",
                        "gif" => "image/gif",
                        "webp" => "image/webp",
                        _ => "image/png",
                    };
                    let b64 = STANDARD.encode(&data);
                    urls.insert(account.credentials.email.clone(), format!("data:{};base64,{}", mime, b64));
                }
            }
        }
    }

    Ok(urls)
}

/// Get everything needed to render on startup - ONE call instead of multiple
#[tauri::command]
fn get_startup_state() -> Result<InitialState, String> {
    get_initial_state().map_err(|e| e.to_string())
}

#[tauri::command]
fn list_emails(account: String, mailbox: String) -> Result<Vec<Email>, String> {
    get_emails(&account, &mailbox).map_err(|e| e.to_string())
}

/// Paginated email list (returns lightweight summaries)
/// Supports virtual sections like "__UNREAD__"
#[tauri::command]
fn list_emails_page(
    account: String,
    mailbox: String,
    offset: usize,
    limit: usize,
) -> Result<ListEmailsResult, String> {
    // Handle virtual sections
    if mailbox == "__UNREAD__" {
        let emails = mail::list_unread_emails(&account).map_err(|e| e.to_string())?;
        let total = emails.len();
        return Ok(ListEmailsResult {
            emails,
            total,
            offset: 0,
            has_more: false,
        });
    }

    list_emails_paginated(&account, &mailbox, offset, limit).map_err(|e| e.to_string())
}

/// Get count of emails within last N days
#[tauri::command]
fn get_email_count_days(account: String, mailbox: String, days: i64) -> Result<usize, String> {
    get_emails_count_since_days(&account, &mailbox, days).map_err(|e| e.to_string())
}

/// Search emails by query string (searches subject, from, snippet)
#[tauri::command]
fn search_emails_cmd(
    account: String,
    mailbox: String,
    query: String,
    limit: usize,
) -> Result<Vec<mail::EmailSummary>, String> {
    mail::search_emails(&account, &mailbox, &query, limit).map_err(|e| e.to_string())
}

#[tauri::command]
fn get_email(account: String, mailbox: String, uid: u32) -> Result<Email, String> {
    fetch_email(&account, &mailbox, uid).map_err(|e| e.to_string())
}

/// Get email with lazy-loaded body - fetches from IMAP if body not cached
/// Use this when opening an email to read its full content
#[tauri::command]
fn get_email_with_body(account: String, mailbox: String, uid: u32) -> Result<Email, String> {
    fetch_email_with_body(&account, &mailbox, uid).map_err(|e| e.to_string())
}

/// Fetch email body asynchronously via connection pool (non-blocking)
/// Returns the body HTML and snippet, or None if not found
#[tauri::command]
async fn fetch_email_body_async(
    account: String,
    mailbox: String,
    uid: u32,
) -> Result<Option<(String, String)>, String> {
    tauri::async_runtime::spawn_blocking(move || {
        imap_queue::fetch_body_via_pool(&account, &mailbox, uid)
            .map_err(|e| e.to_string())
    })
    .await
    .map_err(|e| e.to_string())?
}

/// Update email body in cache (after async fetch)
#[tauri::command]
fn update_email_body_cache(
    account: String,
    mailbox: String,
    uid: u32,
    body_html: String,
    snippet: String,
) -> Result<(), String> {
    mail::update_email_body(&account, &mailbox, uid, &body_html, &snippet)
        .map_err(|e| e.to_string())
}

/// Delete email - optimistic: deletes from cache immediately, queues IMAP for background
#[tauri::command]
fn delete_email(account: String, mailbox: String, uid: u32) -> Result<(), String> {
    // Delete from local cache immediately
    delete_email_from_cache(&account, &mailbox, uid).map_err(|e| e.to_string())?;

    // Queue IMAP move-to-trash for background processing
    queue_move_to_trash(account, mailbox, uid);

    Ok(())
}

/// Permanent delete - bypasses trash, immediately deletes from server
#[tauri::command]
fn permanent_delete_email(account: String, mailbox: String, uid: u32) -> Result<(), String> {
    // Delete from local cache immediately
    delete_email_from_cache(&account, &mailbox, uid).map_err(|e| e.to_string())?;

    // Queue IMAP permanent delete for background processing
    imap_queue::queue_delete(account, mailbox, uid);

    Ok(())
}

#[tauri::command]
fn mark_email_read(account: String, mailbox: String, uid: u32, unread: bool) -> Result<Email, String> {
    update_email_read_status(&account, &mailbox, uid, unread).map_err(|e| e.to_string())
}

/// Mark email read/unread - updates cache immediately, queues IMAP for background
#[tauri::command]
fn mark_email_read_async(account: String, mailbox: String, uid: u32, unread: bool) -> Result<(), String> {
    // Update local cache immediately
    update_email_cache_only(&account, &mailbox, uid, unread).map_err(|e| e.to_string())?;

    // Queue IMAP operation for background processing
    queue_mark_read(account, mailbox, uid, unread);

    Ok(())
}

#[tauri::command]
fn sync_emails(account: String, mailbox: String) -> Result<SyncResult, String> {
    do_sync_emails(&account, &mailbox).map_err(|e| e.to_string())
}

/// Start async sync - queues operation and returns immediately
/// Frontend should listen for sync-started, sync-complete, sync-error events
#[tauri::command]
fn start_sync(account: String, mailbox: String) {
    queue_sync(account, mailbox);
}

/// Get unread counts for all accounts (INBOX)
#[tauri::command]
fn get_all_unread_counts() -> Result<Vec<(String, usize)>, String> {
    fetch_all_unread_counts().map_err(|e| e.to_string())
}

/// Get unread count for a specific mailbox
#[tauri::command]
fn get_unread_count(account: String, mailbox: String) -> Result<usize, String> {
    fetch_unread_count(&account, &mailbox).map_err(|e| e.to_string())
}

/// Get unread counts for all mailboxes of an account
#[tauri::command]
fn get_mailbox_unread_counts(account: String) -> Result<Vec<(String, usize)>, String> {
    fetch_mailbox_unread_counts(&account).map_err(|e| e.to_string())
}

#[tauri::command]
fn get_config() -> Result<Config, String> {
    load_config().map_err(|e| e.to_string())
}

#[tauri::command]
fn save_config(config: Config) -> Result<(), String> {
    store_config(&config).map_err(|e| e.to_string())
}

#[tauri::command]
fn add_ai_provider(provider: AIProvider) -> Result<Config, String> {
    let mut config = load_config().map_err(|e| e.to_string())?;
    config.ai_providers.push(provider);
    store_config(&config).map_err(|e| e.to_string())?;
    Ok(config)
}

#[tauri::command]
fn remove_ai_provider(index: usize) -> Result<Config, String> {
    let mut config = load_config().map_err(|e| e.to_string())?;
    if index < config.ai_providers.len() {
        config.ai_providers.remove(index);
        store_config(&config).map_err(|e| e.to_string())?;
    }
    Ok(config)
}

#[tauri::command]
fn update_ai_provider(index: usize, provider: AIProvider) -> Result<Config, String> {
    let mut config = load_config().map_err(|e| e.to_string())?;
    if index < config.ai_providers.len() {
        config.ai_providers[index] = provider;
        store_config(&config).map_err(|e| e.to_string())?;
    }
    Ok(config)
}

#[tauri::command]
fn save_account_order(order: Vec<String>) -> Result<Config, String> {
    let mut config = load_config().map_err(|e| e.to_string())?;
    config.account_order = order;
    store_config(&config).map_err(|e| e.to_string())?;
    Ok(config)
}

#[tauri::command]
async fn test_telegram(bot_token: String, chat_id: String) -> Result<(), String> {
    tauri::async_runtime::spawn_blocking(move || {
        send_telegram_test(&bot_token, &chat_id)
    })
    .await
    .map_err(|e| e.to_string())?
}

// ============ COMPOSE / SMTP COMMANDS ============

#[tauri::command]
fn send_email(account: String, email: ComposeEmail) -> Result<SendResult, String> {
    smtp_send(&account, email).map_err(|e| e.to_string())
}

// ============ DRAFT COMMANDS ============

#[tauri::command]
fn save_draft(draft: Draft) -> Result<i64, String> {
    mail_save_draft(&draft).map_err(|e| e.to_string())
}

#[tauri::command]
fn get_draft(id: i64) -> Result<Option<Draft>, String> {
    mail_get_draft(id).map_err(|e| e.to_string())
}

#[tauri::command]
fn list_drafts(account: String) -> Result<Vec<Draft>, String> {
    mail_list_drafts(&account).map_err(|e| e.to_string())
}

#[tauri::command]
fn delete_draft(id: i64) -> Result<(), String> {
    mail_delete_draft(id).map_err(|e| e.to_string())
}

/// Sync a local draft to IMAP server's Drafts folder
#[tauri::command]
fn sync_draft_to_server(draft: Draft) -> Result<(), String> {
    let account = draft.account.clone();
    let draft_id = draft.id.unwrap_or(0) as u32;

    // Convert Draft to ComposeEmail
    let attachments: Vec<AttachmentInfo> = if draft.attachments_json.is_empty() {
        vec![]
    } else {
        serde_json::from_str(&draft.attachments_json).unwrap_or_default()
    };

    let compose = ComposeEmail {
        to: draft.to.split(',').map(|s| s.trim().to_string()).filter(|s| !s.is_empty()).collect(),
        cc: draft.cc.split(',').map(|s| s.trim().to_string()).filter(|s| !s.is_empty()).collect(),
        bcc: draft.bcc.split(',').map(|s| s.trim().to_string()).filter(|s| !s.is_empty()).collect(),
        subject: draft.subject,
        body_html: draft.body_html,
        body_text: draft.body_text,
        attachments,
        reply_to_message_id: draft.reply_to_message_id,
        references: None,
    };

    match save_draft_to_imap(&account, &compose) {
        Ok(()) => {
            let _ = log_op(&account, "Drafts", "sync_draft", draft_id, "success", "");
            Ok(())
        }
        Err(e) => {
            let err_msg = e.to_string();
            let _ = log_op(&account, "Drafts", "sync_draft", draft_id, "failed", &err_msg);
            Err(err_msg)
        }
    }
}

// ============ AI COMMANDS ============

#[tauri::command]
fn get_email_summary(account: String, mailbox: String, uid: u32) -> Option<EmailSummary> {
    get_cached_summary(&account, &mailbox, uid)
}

#[tauri::command]
fn delete_email_summary(account: String, mailbox: String, uid: u32) -> Result<(), String> {
    delete_summary(&account, &mailbox, uid).map_err(|e| e.to_string())
}

#[tauri::command]
fn get_available_ai_providers() -> Vec<String> {
    list_available_providers()
}

/// CLI-only completion for frontend JS SDK integration
/// Frontend uses JS SDKs for API providers, only CLI providers go through Rust
#[tauri::command]
async fn cli_complete(
    request: CompletionRequest,
    provider_name: String,
    provider_model: String,
) -> CompletionResponse {
    tauri::async_runtime::spawn_blocking(move || {
        do_cli_complete(request, &provider_name, &provider_model)
    }).await.unwrap_or_else(|_| CompletionResponse {
        success: false,
        content: None,
        error: Some("Task panicked".to_string()),
        model_used: None,
    })
}

/// Save email summary from frontend (after frontend generates it via JS SDK)
#[tauri::command]
fn save_email_summary(
    account: String,
    mailbox: String,
    uid: u32,
    summary: String,
    model_used: String,
) -> Result<(), String> {
    save_summary_from_frontend(&account, &mailbox, uid, &summary, &model_used)
        .map_err(|e| e.to_string())
}

// ============ CALENDAR COMMANDS ============

#[tauri::command]
fn calendar_get_auth_status() -> CalendarAuthStatus {
    cal_auth_status()
}

#[tauri::command]
fn calendar_request_access() -> Result<(), String> {
    cal_request_access().map_err(|e| e.to_string())
}

#[tauri::command]
async fn calendar_list_calendars() -> Result<Vec<CalendarInfo>, String> {
    // Run on background thread to avoid blocking UI
    tokio::task::spawn_blocking(|| {
        cal_list_calendars().map_err(|e| e.to_string())
    })
    .await
    .map_err(|e| e.to_string())?
}

#[tauri::command]
async fn calendar_list_events(start_timestamp: i64, end_timestamp: i64) -> Result<Vec<CalendarEvent>, String> {
    // Run on background thread to avoid blocking UI
    tokio::task::spawn_blocking(move || {
        cal_list_events(start_timestamp, end_timestamp).map_err(|e| e.to_string())
    })
    .await
    .map_err(|e| e.to_string())?
}

#[tauri::command]
async fn calendar_create_event(event: NewEvent) -> Result<String, String> {
    // Run on background thread to avoid blocking UI
    tokio::task::spawn_blocking(move || {
        cal_create_event(&event).map_err(|e| e.to_string())
    })
    .await
    .map_err(|e| e.to_string())?
}

#[tauri::command]
async fn calendar_delete_event(event_id: String) -> Result<(), String> {
    // Run on background thread to avoid blocking UI
    tokio::task::spawn_blocking(move || {
        cal_delete_event(&event_id).map_err(|e| e.to_string())
    })
    .await
    .map_err(|e| e.to_string())?
}

#[tauri::command]
async fn calendar_get_default() -> Result<String, String> {
    // Run on background thread to avoid blocking UI
    tokio::task::spawn_blocking(|| {
        cal_default_calendar().map_err(|e| e.to_string())
    })
    .await
    .map_err(|e| e.to_string())?
}

/// Search calendar events by keyword (searches title, location, notes)
/// Searches within a 1-year range (6 months back, 6 months forward)
#[tauri::command]
async fn calendar_search_events(keyword: String) -> Result<Vec<CalendarEvent>, String> {
    tokio::task::spawn_blocking(move || {
        cal_search_events(&keyword).map_err(|e| e.to_string())
    })
    .await
    .map_err(|e| e.to_string())?
}

// ============ REMINDERS COMMANDS ============

#[tauri::command]
fn reminders_get_auth_status() -> ReminderAuthStatus {
    rem_auth_status()
}

#[tauri::command]
fn reminders_request_access() -> Result<(), String> {
    rem_request_access().map_err(|e| e.to_string())
}

#[tauri::command]
fn reminders_list_lists() -> Result<Vec<ReminderList>, String> {
    rem_list_lists().map_err(|e| e.to_string())
}

#[tauri::command]
fn reminders_list(list_id: Option<String>, include_completed: bool) -> Result<Vec<Reminder>, String> {
    rem_list_reminders(list_id.as_deref(), include_completed).map_err(|e| e.to_string())
}

#[tauri::command]
fn reminders_get(reminder_id: String) -> Result<Reminder, String> {
    rem_get(&reminder_id).map_err(|e| e.to_string())
}

#[tauri::command]
fn reminders_create(reminder: NewReminder) -> Result<String, String> {
    rem_create(&reminder).map_err(|e| e.to_string())
}

/// Create reminder from email - the main use case
#[tauri::command]
async fn reminders_create_from_email(
    email_subject: String,
    email_from: String,
    email_body: String,
    due_date: Option<i64>,
    priority: Option<i32>,
    list_id: Option<String>,
) -> Result<String, String> {
    tauri::async_runtime::spawn_blocking(move || {
        let req = ReminderFromEmail {
            email_subject,
            email_from,
            email_body,
            due_date,
            priority: priority.unwrap_or(0),
            list_id: list_id.unwrap_or_default(),
        };
        rem_create_from_email(&req).map_err(|e| e.to_string())
    })
    .await
    .map_err(|e| e.to_string())?
}

#[tauri::command]
fn reminders_update(
    reminder_id: String,
    title: Option<String>,
    notes: Option<String>,
    due_date: Option<i64>,
    clear_due_date: Option<bool>,
    priority: Option<i32>,
) -> Result<(), String> {
    rem_update(
        &reminder_id,
        title.as_deref(),
        notes.as_deref(),
        due_date,
        clear_due_date.unwrap_or(false),
        priority.unwrap_or(0),
    ).map_err(|e| e.to_string())
}

#[tauri::command]
fn reminders_delete(reminder_id: String) -> Result<(), String> {
    rem_delete(&reminder_id).map_err(|e| e.to_string())
}

#[tauri::command]
fn reminders_complete(reminder_id: String) -> Result<(), String> {
    rem_complete(&reminder_id).map_err(|e| e.to_string())
}

#[tauri::command]
fn reminders_uncomplete(reminder_id: String) -> Result<(), String> {
    rem_uncomplete(&reminder_id).map_err(|e| e.to_string())
}

#[tauri::command]
fn reminders_get_default_list() -> Result<String, String> {
    rem_default_list().map_err(|e| e.to_string())
}

#[tauri::command]
fn reminders_search(query: String, list_id: Option<String>) -> Result<Vec<Reminder>, String> {
    rem_search(&query, list_id.as_deref()).map_err(|e| e.to_string())
}

// ============ TAG OPERATIONS ============

#[tauri::command]
fn list_tags() -> Result<Vec<mail::Tag>, String> {
    mail::get_all_tags().map_err(|e| e.to_string())
}

#[tauri::command]
fn create_tag(name: String, color: String) -> Result<mail::Tag, String> {
    mail::create_tag(&name, &color).map_err(|e| e.to_string())
}

#[tauri::command]
fn delete_tag(tag_id: i64) -> Result<(), String> {
    mail::delete_tag(tag_id).map_err(|e| e.to_string())
}

#[tauri::command]
fn update_tag(tag_id: i64, name: String, color: String) -> Result<mail::Tag, String> {
    mail::update_tag(tag_id, &name, &color).map_err(|e| e.to_string())
}

#[tauri::command]
fn get_email_tags(account: String, mailbox: String, uid: u32) -> Result<Vec<mail::EmailTag>, String> {
    mail::get_email_tags(&account, &mailbox, uid).map_err(|e| e.to_string())
}

#[tauri::command]
fn add_email_tag(account: String, mailbox: String, uid: u32, tag_id: i64, auto_generated: bool) -> Result<(), String> {
    mail::add_tag_to_email(&account, &mailbox, uid, tag_id, auto_generated).map_err(|e| e.to_string())
}

#[tauri::command]
fn remove_email_tag(account: String, mailbox: String, uid: u32, tag_id: i64) -> Result<(), String> {
    mail::remove_tag_from_email(&account, &mailbox, uid, tag_id).map_err(|e| e.to_string())
}

#[tauri::command]
fn get_batch_email_tags(account: String, mailbox: String, uids: Vec<u32>) -> Result<std::collections::HashMap<u32, Vec<mail::EmailTag>>, String> {
    mail::get_emails_tags_batch(&account, &mailbox, &uids).map_err(|e| e.to_string())
}

#[tauri::command]
fn search_emails_by_tags(account: String, mailbox: String, tag_ids: Vec<i64>) -> Result<Vec<u32>, String> {
    mail::search_emails_by_tags(&account, &mailbox, &tag_ids).map_err(|e| e.to_string())
}

#[cfg_attr(mobile, tauri::mobile_entry_point)]
pub fn run() {
    // Initialize tokio runtime for background tasks
    let runtime = tokio::runtime::Builder::new_multi_thread()
        .enable_all()
        .build()
        .expect("Failed to create tokio runtime");

    // Enter the runtime context so tokio::spawn works
    let _guard = runtime.enter();

    tauri::Builder::default()
        .plugin(tauri_plugin_opener::init())
        .plugin(tauri_plugin_dialog::init())
        .plugin(tauri_plugin_updater::Builder::new().build())
        .plugin(tauri_plugin_process::init())
        .plugin(tauri_plugin_notification::init())
        .plugin(tauri_plugin_http::init())
        .setup(|app| {
            // Initialize database eagerly
            init_db();
            // Initialize summaries table
            init_summaries_table();

            // Get initial state for injection
            let initial_state = get_initial_state().ok();
            let init_script = match initial_state {
                Some(state) => {
                    let json = serde_json::to_string(&state).unwrap_or_default();
                    format!("window.__MAILY_INITIAL_STATE__ = {};", json)
                }
                None => String::new(),
            };

            // Create window with initialization script (data injected before React loads)
            let mut builder = WebviewWindowBuilder::new(
                app,
                "main",
                tauri::WebviewUrl::App("index.html".into()),
            )
            .title("Maily")
            .inner_size(1200.0, 800.0)
            .devtools(true); // Enable DevTools in release builds (Cmd+Option+I)

            if !init_script.is_empty() {
                builder = builder.initialization_script(&init_script);
            }

            let window = builder.build()?;

            // On macOS: hide window instead of closing when user clicks the red X button
            // This keeps the app running in the background for email sync
            #[cfg(target_os = "macos")]
            {
                let window_clone = window.clone();
                window.on_window_event(move |event| {
                    if let WindowEvent::CloseRequested { api, .. } = event {
                        // Prevent the window from being destroyed
                        api.prevent_close();
                        // Hide the window instead
                        let _ = window_clone.hide();
                    }
                });
            }

            // Create system tray (menu bar icon) - click to show window
            let tray_icon = Image::from_bytes(include_bytes!("../icons/32x32.png"))?;

            let _tray = TrayIconBuilder::new()
                .icon(tray_icon)
                .tooltip("Maily")
                .on_tray_icon_event(|tray, event| {
                    // Any click on tray icon shows the window
                    if let tauri::tray::TrayIconEvent::Click { .. } = event {
                        if let Some(window) = tray.app_handle().get_webview_window("main") {
                            let _ = window.show();
                            let _ = window.set_focus();
                        }
                    }
                })
                .build(app)?;

            // Initialize IMAP queue with app handle for events
            init_imap_queue(app.handle().clone());

            // Start background sync timer (every 10 minutes)
            tokio::spawn(async {
                // Wait 1 minute before first sync (let app fully initialize)
                tokio::time::sleep(std::time::Duration::from_secs(60)).await;

                let mut interval = tokio::time::interval(std::time::Duration::from_secs(600)); // 10 minutes
                loop {
                    interval.tick().await;
                    eprintln!("[sync-timer] Starting background sync for all accounts");

                    if let Ok(accounts) = mail::get_accounts() {
                        for account in accounts {
                            eprintln!("[sync-timer] Queuing sync for {}", account.name);
                            imap_queue::queue_sync(account.name.clone(), "INBOX".to_string());
                        }
                    }
                }
            });

            Ok(())
        })
        .invoke_handler(tauri::generate_handler![
            // Account operations
            list_full_accounts,
            add_account,
            remove_account,
            update_account,
            test_account,
            upload_account_avatar,
            delete_account_avatar,
            get_account_avatar_urls,
            // Email operations
            get_startup_state,
            list_emails,
            list_emails_page,
            get_email_count_days,
            search_emails_cmd,
            get_email,
            get_email_with_body,
            fetch_email_body_async,
            update_email_body_cache,
            delete_email,
            permanent_delete_email,
            mark_email_read,
            mark_email_read_async,
            sync_emails,
            start_sync,
            get_all_unread_counts,
            get_unread_count,
            get_mailbox_unread_counts,
            // Config operations
            get_config,
            save_config,
            add_ai_provider,
            remove_ai_provider,
            update_ai_provider,
            save_account_order,
            test_telegram,
            // Compose / SMTP
            send_email,
            // Drafts
            save_draft,
            get_draft,
            list_drafts,
            delete_draft,
            sync_draft_to_server,
            // AI operations (API calls handled by frontend JS SDKs, only CLI goes through Rust)
            get_email_summary,
            delete_email_summary,
            get_available_ai_providers,
            cli_complete,
            save_email_summary,
            // Calendar operations
            calendar_get_auth_status,
            calendar_request_access,
            calendar_list_calendars,
            calendar_list_events,
            calendar_create_event,
            calendar_delete_event,
            calendar_get_default,
            calendar_search_events,
            // Reminders operations
            reminders_get_auth_status,
            reminders_request_access,
            reminders_list_lists,
            reminders_list,
            reminders_get,
            reminders_create,
            reminders_create_from_email,
            reminders_update,
            reminders_delete,
            reminders_complete,
            reminders_uncomplete,
            reminders_get_default_list,
            reminders_search,
            // Tag operations
            list_tags,
            create_tag,
            delete_tag,
            update_tag,
            get_email_tags,
            add_email_tag,
            remove_email_tag,
            get_batch_email_tags,
            search_emails_by_tags
        ])
        .build(tauri::generate_context!())
        .expect("error while building tauri application")
        .run(|app_handle, event| {
            // On macOS: show window when user clicks the dock icon
            #[cfg(target_os = "macos")]
            if let RunEvent::Reopen { has_visible_windows, .. } = event {
                if !has_visible_windows {
                    if let Some(window) = app_handle.get_webview_window("main") {
                        let _ = window.show();
                        let _ = window.set_focus();
                    }
                }
            }
        });
}
