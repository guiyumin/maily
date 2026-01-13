use serde::{Deserialize, Serialize};
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
        }
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
