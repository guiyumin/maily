mod config;
mod imap_queue;
mod mail;

use config::{get_config as load_config, save_config as store_config, AIProvider, Config};
use imap_queue::queue_mark_read;
use mail::{
    delete_email_from_cache, get_accounts, get_email as fetch_email, get_emails,
    sync_emails as do_sync_emails, update_email_read_status, update_email_cache_only,
    Account, Email, SyncResult,
};

#[tauri::command]
fn list_accounts() -> Result<Vec<Account>, String> {
    get_accounts().map_err(|e| e.to_string())
}

#[tauri::command]
fn list_emails(account: String, mailbox: String) -> Result<Vec<Email>, String> {
    get_emails(&account, &mailbox).map_err(|e| e.to_string())
}

#[tauri::command]
fn get_email(account: String, mailbox: String, uid: u32) -> Result<Email, String> {
    fetch_email(&account, &mailbox, uid).map_err(|e| e.to_string())
}

#[tauri::command]
fn delete_email(account: String, mailbox: String, uid: u32) -> Result<(), String> {
    delete_email_from_cache(&account, &mailbox, uid).map_err(|e| e.to_string())
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
    tauri::Builder::default()
        .plugin(tauri_plugin_opener::init())
        .invoke_handler(tauri::generate_handler![
            list_accounts,
            list_emails,
            get_email,
            delete_email,
            mark_email_read,
            mark_email_read_async,
            sync_emails,
            get_config,
            save_config,
            add_ai_provider,
            remove_ai_provider
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
