mod config;
mod imap_queue;
mod mail;

use tauri::WebviewWindowBuilder;
use config::{get_config as load_config, save_config as store_config, AIProvider, Config};
use imap_queue::{init as init_imap_queue, queue_mark_read, queue_move_to_trash, queue_sync};
use mail::{
    delete_email_from_cache, get_accounts, get_email as fetch_email, get_emails,
    list_emails_paginated, get_emails_count_since_days, init_db, get_initial_state,
    sync_emails as do_sync_emails, update_email_read_status, update_email_cache_only,
    Account, Email, ListEmailsResult, SyncResult, InitialState,
};

#[tauri::command]
fn list_accounts() -> Result<Vec<Account>, String> {
    get_accounts().map_err(|e| e.to_string())
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
        .setup(|app| {
            // Initialize database eagerly
            init_db();

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

            Ok(())
        })
        .invoke_handler(tauri::generate_handler![
            list_accounts,
            get_startup_state,
            list_emails,
            list_emails_page,
            get_email_count_days,
            get_email,
            delete_email,
            mark_email_read,
            mark_email_read_async,
            sync_emails,
            start_sync,
            get_config,
            save_config,
            add_ai_provider,
            remove_ai_provider
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
