mod ai;
mod calendar;
mod config;
mod imap_queue;
mod mail;
mod reminders;
mod smtp;

use tauri::WebviewWindowBuilder;
use ai::{
    complete as do_ai_complete, init_summaries_table, summarize_email as ai_summarize,
    generate_reply as ai_generate_reply, extract_event as ai_extract_event,
    extract_reminder as ai_extract_reminder, parse_event_nlp as ai_parse_event_nlp,
    get_cached_summary, delete_summary, list_available_providers, test_provider as ai_test_provider,
    CompletionRequest, CompletionResponse, EmailSummary,
};
use config::{get_config as load_config, save_config as store_config, send_telegram_test, AIProvider, Config};
use imap_queue::{init as init_imap_queue, queue_mark_read, queue_move_to_trash, queue_sync};
use mail::{
    delete_email_from_cache, get_accounts, get_email as fetch_email, get_emails,
    list_emails_paginated, get_emails_count_since_days, init_db, get_initial_state,
    sync_emails as do_sync_emails, update_email_read_status, update_email_cache_only,
    get_all_unread_counts as fetch_all_unread_counts,
    get_unread_count as fetch_unread_count,
    get_mailbox_unread_counts as fetch_mailbox_unread_counts,
    save_draft as mail_save_draft, get_draft as mail_get_draft,
    list_drafts as mail_list_drafts, delete_draft as mail_delete_draft, log_op,
    add_account as mail_add_account, remove_account as mail_remove_account,
    update_account as mail_update_account, test_account_credentials,
    Account, Email, ListEmailsResult, SyncResult, InitialState, Draft,
};
use smtp::{send_email as smtp_send, save_draft_to_imap, ComposeEmail, SendResult, AttachmentInfo};
use calendar::{
    AuthStatus as CalendarAuthStatus, Calendar as CalendarInfo, Event as CalendarEvent,
    NewEvent, get_auth_status as cal_auth_status, request_access as cal_request_access,
    list_calendars as cal_list_calendars, list_events as cal_list_events,
    create_event as cal_create_event, delete_event as cal_delete_event,
    get_default_calendar as cal_default_calendar,
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
fn list_accounts() -> Result<Vec<Account>, String> {
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
#[tauri::command]
fn list_emails_page(
    account: String,
    mailbox: String,
    offset: usize,
    limit: usize,
) -> Result<ListEmailsResult, String> {
    list_emails_paginated(&account, &mailbox, offset, limit).map_err(|e| e.to_string())
}

/// Get count of emails within last N days
#[tauri::command]
fn get_email_count_days(account: String, mailbox: String, days: i64) -> Result<usize, String> {
    get_emails_count_since_days(&account, &mailbox, days).map_err(|e| e.to_string())
}

#[tauri::command]
fn get_email(account: String, mailbox: String, uid: u32) -> Result<Email, String> {
    fetch_email(&account, &mailbox, uid).map_err(|e| e.to_string())
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

// Use spawn_blocking for CPU-heavy/blocking AI calls to not block async runtime
#[tauri::command]
async fn summarize_email(
    account: String,
    mailbox: String,
    uid: u32,
    subject: String,
    from: String,
    body_text: String,
    force_refresh: bool,
) -> CompletionResponse {
    tauri::async_runtime::spawn_blocking(move || {
        ai_summarize(&account, &mailbox, uid, &subject, &from, &body_text, force_refresh)
    }).await.unwrap_or_else(|_| CompletionResponse {
        success: false,
        content: None,
        error: Some("Task panicked".to_string()),
        model_used: None,
    })
}

#[tauri::command]
fn get_email_summary(account: String, mailbox: String, uid: u32) -> Option<EmailSummary> {
    get_cached_summary(&account, &mailbox, uid)
}

#[tauri::command]
fn delete_email_summary(account: String, mailbox: String, uid: u32) -> Result<(), String> {
    delete_summary(&account, &mailbox, uid).map_err(|e| e.to_string())
}

#[tauri::command]
async fn generate_reply(
    original_from: String,
    original_subject: String,
    original_body: String,
    reply_intent: String,
) -> CompletionResponse {
    tauri::async_runtime::spawn_blocking(move || {
        ai_generate_reply(&original_from, &original_subject, &original_body, &reply_intent)
    }).await.unwrap_or_else(|_| CompletionResponse {
        success: false,
        content: None,
        error: Some("Task panicked".to_string()),
        model_used: None,
    })
}

#[tauri::command]
async fn extract_event(from: String, subject: String, body_text: String) -> CompletionResponse {
    tauri::async_runtime::spawn_blocking(move || {
        ai_extract_event(&from, &subject, &body_text)
    }).await.unwrap_or_else(|_| CompletionResponse {
        success: false,
        content: None,
        error: Some("Task panicked".to_string()),
        model_used: None,
    })
}

#[tauri::command]
async fn extract_reminder(from: String, subject: String, body_text: String) -> CompletionResponse {
    tauri::async_runtime::spawn_blocking(move || {
        ai_extract_reminder(&from, &subject, &body_text)
    }).await.unwrap_or_else(|_| CompletionResponse {
        success: false,
        content: None,
        error: Some("Task panicked".to_string()),
        model_used: None,
    })
}

#[tauri::command]
async fn parse_event_nlp(
    user_input: String,
    email_from: String,
    email_subject: String,
    email_body: String,
) -> CompletionResponse {
    tauri::async_runtime::spawn_blocking(move || {
        ai_parse_event_nlp(&user_input, &email_from, &email_subject, &email_body)
    }).await.unwrap_or_else(|_| CompletionResponse {
        success: false,
        content: None,
        error: Some("Task panicked".to_string()),
        model_used: None,
    })
}

#[tauri::command]
async fn ai_complete(request: CompletionRequest) -> CompletionResponse {
    tauri::async_runtime::spawn_blocking(move || {
        do_ai_complete(request)
    }).await.unwrap_or_else(|_| CompletionResponse {
        success: false,
        content: None,
        error: Some("Task panicked".to_string()),
        model_used: None,
    })
}

#[tauri::command]
fn get_available_ai_providers() -> Vec<String> {
    list_available_providers()
}

#[tauri::command]
async fn test_ai_provider(
    provider_name: String,
    provider_model: String,
    provider_type: String,
    base_url: String,
    api_key: String,
) -> CompletionResponse {
    tauri::async_runtime::spawn_blocking(move || {
        ai_test_provider(&provider_name, &provider_model, &provider_type, &base_url, &api_key)
    }).await.unwrap_or_else(|_| CompletionResponse {
        success: false,
        content: None,
        error: Some("Task panicked".to_string()),
        model_used: None,
    })
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
fn calendar_list_calendars() -> Result<Vec<CalendarInfo>, String> {
    cal_list_calendars().map_err(|e| e.to_string())
}

#[tauri::command]
fn calendar_list_events(start_timestamp: i64, end_timestamp: i64) -> Result<Vec<CalendarEvent>, String> {
    cal_list_events(start_timestamp, end_timestamp).map_err(|e| e.to_string())
}

#[tauri::command]
fn calendar_create_event(event: NewEvent) -> Result<String, String> {
    cal_create_event(&event).map_err(|e| e.to_string())
}

#[tauri::command]
fn calendar_delete_event(event_id: String) -> Result<(), String> {
    cal_delete_event(&event_id).map_err(|e| e.to_string())
}

#[tauri::command]
fn calendar_get_default() -> Result<String, String> {
    cal_default_calendar().map_err(|e| e.to_string())
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
            .inner_size(1200.0, 800.0);

            if !init_script.is_empty() {
                builder = builder.initialization_script(&init_script);
            }

            builder.build()?;

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
            list_accounts,
            add_account,
            remove_account,
            update_account,
            test_account,
            // Email operations
            get_startup_state,
            list_emails,
            list_emails_page,
            get_email_count_days,
            get_email,
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
            // AI operations
            summarize_email,
            get_email_summary,
            delete_email_summary,
            generate_reply,
            extract_event,
            extract_reminder,
            parse_event_nlp,
            ai_complete,
            get_available_ai_providers,
            test_ai_provider,
            // Calendar operations
            calendar_get_auth_status,
            calendar_request_access,
            calendar_list_calendars,
            calendar_list_events,
            calendar_create_event,
            calendar_delete_event,
            calendar_get_default,
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
            reminders_search
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
