use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::fs;
use std::path::PathBuf;

#[derive(Debug, Serialize, Deserialize, Clone, PartialEq)]
#[serde(rename_all = "lowercase")]
pub enum AIProviderType {
    Cli,
    Api,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct AIProvider {
    #[serde(rename = "type")]
    pub provider_type: AIProviderType,
    pub name: String,
    pub model: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub base_url: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub api_key: String,
    /// SDK to use for API calls: "openai" (default), "anthropic"
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub sdk: Option<String>,
    /// Custom HTTP headers for API calls
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub custom_headers: Option<HashMap<String, String>>,
}

// Notification settings
#[derive(Debug, Serialize, Deserialize, Clone, Default)]
pub struct NativeNotificationConfig {
    #[serde(default = "default_true")]
    pub enabled: bool,
    #[serde(default = "default_true")]
    pub new_email: bool,
    #[serde(default = "default_true")]
    pub calendar_reminder: bool,
}

#[derive(Debug, Serialize, Deserialize, Clone, Default)]
pub struct TelegramConfig {
    #[serde(default)]
    pub enabled: bool,
    #[serde(default)]
    pub bot_token: String,
    #[serde(default)]
    pub chat_id: String,
}

#[derive(Debug, Serialize, Deserialize, Clone, Default)]
pub struct NotificationConfig {
    #[serde(default)]
    pub native: NativeNotificationConfig,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub telegram: Option<TelegramConfig>,
}

// Integration settings
#[derive(Debug, Serialize, Deserialize, Clone, Default)]
pub struct GitHubConfig {
    #[serde(default)]
    pub enabled: bool,
    #[serde(default)]
    pub token: String,
    #[serde(default = "default_true")]
    pub parse_emails: bool,
}

#[derive(Debug, Serialize, Deserialize, Clone, Default)]
pub struct IntegrationsConfig {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub github: Option<GitHubConfig>,
}

fn default_true() -> bool {
    true
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct Config {
    #[serde(default = "default_max_emails")]
    pub max_emails: i32,
    #[serde(default = "default_label")]
    pub default_label: String,
    #[serde(default = "default_theme")]
    pub theme: String,
    #[serde(default, skip_serializing_if = "String::is_empty")]
    pub language: String,
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub ai_providers: Vec<AIProvider>,
    /// Order of accounts (account names). First 3 are visible in sidebar.
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub account_order: Vec<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub notifications: Option<NotificationConfig>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub integrations: Option<IntegrationsConfig>,
}

fn default_max_emails() -> i32 {
    50
}

fn default_label() -> String {
    "INBOX".to_string()
}

fn default_theme() -> String {
    "default".to_string()
}

impl Default for Config {
    fn default() -> Self {
        Config {
            max_emails: default_max_emails(),
            default_label: default_label(),
            theme: default_theme(),
            language: String::new(),
            ai_providers: Vec::new(),
            account_order: Vec::new(),
            notifications: None,
            integrations: None,
        }
    }
}

/// Send a test message to Telegram
pub fn send_telegram_test(bot_token: &str, chat_id: &str) -> Result<(), String> {
    let url = format!(
        "https://api.telegram.org/bot{}/sendMessage",
        bot_token
    );

    let client = reqwest::blocking::Client::new();
    let response = client
        .post(&url)
        .json(&serde_json::json!({
            "chat_id": chat_id,
            "text": "🎉 <b>Maily</b> test notification!\n\nYour Telegram integration is working.",
            "parse_mode": "HTML"
        }))
        .send()
        .map_err(|e| e.to_string())?;

    if response.status().is_success() {
        Ok(())
    } else {
        let error_text = response.text().unwrap_or_default();
        Err(format!("Telegram API error: {}", error_text))
    }
}

fn config_dir() -> PathBuf {
    dirs::home_dir()
        .expect("Could not find home directory")
        .join(".config")
        .join("maily")
}

pub fn get_config() -> Result<Config, Box<dyn std::error::Error>> {
    let config_path = config_dir().join("config.yml");

    if !config_path.exists() {
        return Ok(Config::default());
    }

    let contents = fs::read_to_string(&config_path)?;
    let config: Config = serde_yaml::from_str(&contents)?;
    Ok(config)
}

pub fn save_config(config: &Config) -> Result<(), Box<dyn std::error::Error>> {
    let config_dir = config_dir();
    fs::create_dir_all(&config_dir)?;

    let config_path = config_dir.join("config.yml");
    let contents = serde_yaml::to_string(config)?;
    fs::write(&config_path, contents)?;

    Ok(())
}
