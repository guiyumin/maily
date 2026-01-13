mod config;
mod mail;

use config::{get_config as load_config, save_config as store_config, AIProvider, Config};
use mail::{get_accounts, get_emails, Account, Email};

#[tauri::command]
fn list_accounts() -> Result<Vec<Account>, String> {
    get_accounts().map_err(|e| e.to_string())
}

#[tauri::command]
fn list_emails(account: String, mailbox: String) -> Result<Vec<Email>, String> {
    get_emails(&account, &mailbox).map_err(|e| e.to_string())
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
            get_config,
            save_config,
            add_ai_provider,
            remove_ai_provider
        ])
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
