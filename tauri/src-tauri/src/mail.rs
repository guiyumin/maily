use imap::Session;
use native_tls::TlsStream;
use serde::{Deserialize, Serialize};
use std::fs;
use std::net::TcpStream;
use std::path::PathBuf;

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct Credentials {
    pub email: String,
    #[serde(skip_serializing)]
    #[allow(dead_code)]
    pub password: String,
    pub imap_host: String,
    pub imap_port: u16,
    pub smtp_host: String,
    pub smtp_port: u16,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct Account {
    pub name: String,
    pub provider: String,
    pub credentials: Credentials,
}

#[derive(Debug, Serialize, Deserialize)]
struct AccountsFile {
    accounts: Vec<Account>,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct Attachment {
    pub part_id: String,
    pub filename: String,
    pub content_type: String,
    pub size: i64,
    #[serde(default)]
    pub encoding: String,
}

#[derive(Debug, Serialize, Deserialize, Clone)]
pub struct Email {
    pub uid: u32,
    pub message_id: String,
    pub internal_date: String,
    pub from: String,
    #[serde(default)]
    pub reply_to: String,
    pub to: String,
    #[serde(default)]
    pub cc: String,
    pub subject: String,
    pub date: String,
    #[serde(default)]
    pub snippet: String,
    #[serde(default)]
    pub body_html: String,
    #[serde(default)]
    pub unread: bool,
    #[serde(default)]
    pub attachments: Vec<Attachment>,
}

fn config_dir() -> PathBuf {
    dirs::home_dir()
        .expect("Could not find home directory")
        .join(".config")
        .join("maily")
}

pub fn get_accounts() -> Result<Vec<Account>, Box<dyn std::error::Error>> {
    let accounts_path = config_dir().join("accounts.yml");
    let contents = fs::read_to_string(&accounts_path)?;
    let accounts_file: AccountsFile = serde_yaml::from_str(&contents)?;
    Ok(accounts_file.accounts)
}

fn get_account(name: &str) -> Result<Account, Box<dyn std::error::Error>> {
    let accounts = get_accounts()?;
    accounts
        .into_iter()
        .find(|a| a.name == name)
        .ok_or_else(|| format!("Account '{}' not found", name).into())
}

fn connect_imap(account: &Account) -> Result<Session<TlsStream<TcpStream>>, Box<dyn std::error::Error>> {
    let creds = &account.credentials;
    let tls = native_tls::TlsConnector::builder().build()?;
    let client = imap::connect((creds.imap_host.as_str(), creds.imap_port), &creds.imap_host, &tls)?;
    let session = client.login(&creds.email, &creds.password)
        .map_err(|e| e.0)?;
    Ok(session)
}

pub fn get_emails(account: &str, mailbox: &str) -> Result<Vec<Email>, Box<dyn std::error::Error>> {
    let cache_dir = config_dir().join("cache").join(account).join(mailbox);

    if !cache_dir.exists() {
        return Ok(vec![]);
    }

    let mut emails: Vec<Email> = vec![];

    for entry in fs::read_dir(&cache_dir)? {
        let entry = entry?;
        let path = entry.path();

        if path.extension().map_or(false, |ext| ext == "json") {
            let contents = fs::read_to_string(&path)?;
            if let Ok(email) = serde_json::from_str::<Email>(&contents) {
                emails.push(email);
            }
        }
    }

    // Sort by UID descending (newest first)
    emails.sort_by(|a, b| b.uid.cmp(&a.uid));

    Ok(emails)
}

pub fn get_email(account: &str, mailbox: &str, uid: u32) -> Result<Email, Box<dyn std::error::Error>> {
    let cache_path = config_dir()
        .join("cache")
        .join(account)
        .join(mailbox)
        .join(format!("{}.json", uid));

    if !cache_path.exists() {
        return Err(format!("Email {} not found in cache", uid).into());
    }

    let contents = fs::read_to_string(&cache_path)?;
    let email: Email = serde_json::from_str(&contents)?;
    Ok(email)
}

pub fn delete_email_from_cache(account: &str, mailbox: &str, uid: u32) -> Result<(), Box<dyn std::error::Error>> {
    let cache_path = config_dir()
        .join("cache")
        .join(account)
        .join(mailbox)
        .join(format!("{}.json", uid));

    if cache_path.exists() {
        fs::remove_file(&cache_path)?;
    }
    Ok(())
}

pub fn update_email_read_status(account_name: &str, mailbox: &str, uid: u32, unread: bool) -> Result<Email, Box<dyn std::error::Error>> {
    let cache_path = config_dir()
        .join("cache")
        .join(account_name)
        .join(mailbox)
        .join(format!("{}.json", uid));

    if !cache_path.exists() {
        return Err(format!("Email {} not found in cache", uid).into());
    }

    // Update IMAP server first
    let account = get_account(account_name)?;
    let mut session = connect_imap(&account)?;
    session.select(mailbox)?;

    let uid_set = format!("{}", uid);
    if unread {
        // Remove \Seen flag to mark as unread
        session.uid_store(&uid_set, "-FLAGS (\\Seen)")?;
    } else {
        // Add \Seen flag to mark as read
        session.uid_store(&uid_set, "+FLAGS (\\Seen)")?;
    }

    session.logout()?;

    // Update local cache
    let contents = fs::read_to_string(&cache_path)?;
    let mut email: Email = serde_json::from_str(&contents)?;
    email.unread = unread;

    let updated = serde_json::to_string_pretty(&email)?;
    fs::write(&cache_path, updated)?;

    Ok(email)
}
