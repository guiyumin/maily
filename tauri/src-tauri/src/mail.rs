use imap::Session;
use mailparse::{parse_mail, MailHeaderMap};
use native_tls::TlsStream;
use serde::{Deserialize, Serialize};
use std::collections::HashSet;
use std::fs;
use std::net::TcpStream;
use std::path::PathBuf;
use std::time::Duration;

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

/// Lightweight email summary for list view (no body)
#[derive(Debug, Serialize, Clone)]
pub struct EmailSummary {
    pub uid: u32,
    pub message_id: String,
    pub internal_date: String,
    pub from: String,
    pub to: String,
    pub subject: String,
    pub date: String,
    pub snippet: String,
    pub unread: bool,
    pub has_attachments: bool,
}

impl From<&Email> for EmailSummary {
    fn from(email: &Email) -> Self {
        EmailSummary {
            uid: email.uid,
            message_id: email.message_id.clone(),
            internal_date: email.internal_date.clone(),
            from: email.from.clone(),
            to: email.to.clone(),
            subject: email.subject.clone(),
            date: email.date.clone(),
            snippet: email.snippet.clone(),
            unread: email.unread,
            has_attachments: !email.attachments.is_empty(),
        }
    }
}

/// Result of paginated email list
#[derive(Debug, Serialize, Clone)]
pub struct ListEmailsResult {
    pub emails: Vec<EmailSummary>,
    pub total: usize,
    pub offset: usize,
    pub has_more: bool,
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
    use std::net::ToSocketAddrs;

    let creds = &account.credentials;

    // Resolve hostname to IP address
    let addr = format!("{}:{}", creds.imap_host, creds.imap_port);
    let socket_addr = addr
        .to_socket_addrs()?
        .next()
        .ok_or("Failed to resolve IMAP host")?;

    // Connect with timeout
    let tcp = TcpStream::connect_timeout(&socket_addr, Duration::from_secs(30))?;
    tcp.set_read_timeout(Some(Duration::from_secs(60)))?;
    tcp.set_write_timeout(Some(Duration::from_secs(30)))?;

    let tls = native_tls::TlsConnector::builder().build()?;
    let tls_stream = tls.connect(&creds.imap_host, tcp)?;

    let client = imap::Client::new(tls_stream);
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

/// Get paginated email list (lightweight summaries)
pub fn list_emails_paginated(
    account: &str,
    mailbox: &str,
    offset: usize,
    limit: usize,
) -> Result<ListEmailsResult, Box<dyn std::error::Error>> {
    let cache_dir = config_dir().join("cache").join(account).join(mailbox);

    if !cache_dir.exists() {
        return Ok(ListEmailsResult {
            emails: vec![],
            total: 0,
            offset,
            has_more: false,
        });
    }

    // Read all emails (we need to sort by date)
    // TODO: Optimize with index file for large mailboxes
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

    // Sort by internal_date descending (newest first)
    emails.sort_by(|a, b| b.internal_date.cmp(&a.internal_date));

    let total = emails.len();
    let has_more = offset + limit < total;

    // Get the requested slice and convert to summaries
    let summaries: Vec<EmailSummary> = emails
        .iter()
        .skip(offset)
        .take(limit)
        .map(EmailSummary::from)
        .collect();

    Ok(ListEmailsResult {
        emails: summaries,
        total,
        offset,
        has_more,
    })
}

/// Count emails within the last N days
pub fn get_emails_count_since_days(
    account: &str,
    mailbox: &str,
    days: i64,
) -> Result<usize, Box<dyn std::error::Error>> {
    let cache_dir = config_dir().join("cache").join(account).join(mailbox);

    if !cache_dir.exists() {
        return Ok(0);
    }

    let cutoff = chrono::Utc::now() - chrono::Duration::days(days);
    let mut count = 0;

    for entry in fs::read_dir(&cache_dir)? {
        let entry = entry?;
        let path = entry.path();

        if path.extension().map_or(false, |ext| ext == "json") {
            let contents = fs::read_to_string(&path)?;
            if let Ok(email) = serde_json::from_str::<Email>(&contents) {
                // Parse internal_date and compare
                if let Ok(date) = chrono::DateTime::parse_from_rfc3339(&email.internal_date) {
                    if date >= cutoff {
                        count += 1;
                    }
                }
            }
        }
    }

    Ok(count)
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

/// Update only the local cache (no IMAP) - used with background queue
pub fn update_email_cache_only(account_name: &str, mailbox: &str, uid: u32, unread: bool) -> Result<Email, Box<dyn std::error::Error>> {
    let cache_path = config_dir()
        .join("cache")
        .join(account_name)
        .join(mailbox)
        .join(format!("{}.json", uid));

    if !cache_path.exists() {
        return Err(format!("Email {} not found in cache", uid).into());
    }

    let contents = fs::read_to_string(&cache_path)?;
    let mut email: Email = serde_json::from_str(&contents)?;
    email.unread = unread;

    let updated = serde_json::to_string_pretty(&email)?;
    fs::write(&cache_path, updated)?;

    Ok(email)
}

#[derive(Debug, Serialize, Clone)]
pub struct SyncResult {
    pub new_emails: usize,
    pub updated_emails: usize,
    pub total_emails: usize,
}

pub fn sync_emails(account_name: &str, mailbox: &str) -> Result<SyncResult, Box<dyn std::error::Error>> {
    eprintln!("[sync] Starting sync for {} / {}", account_name, mailbox);

    let account = get_account(account_name)?;
    eprintln!("[sync] Connecting to IMAP...");
    let mut session = connect_imap(&account)?;
    eprintln!("[sync] Connected!");

    // Select mailbox
    eprintln!("[sync] Selecting mailbox...");
    let mailbox_info = session.select(mailbox)?;
    let total = mailbox_info.exists as usize;
    eprintln!("[sync] Mailbox has {} messages", total);

    if total == 0 {
        session.logout()?;
        return Ok(SyncResult {
            new_emails: 0,
            updated_emails: 0,
            total_emails: 0,
        });
    }

    // Fetch all UIDs and flags
    eprintln!("[sync] Fetching UIDs and flags...");
    let fetch_range = "1:*";
    let messages = session.fetch(fetch_range, "(UID FLAGS)")?;
    eprintln!("[sync] Got {} messages", messages.len());

    // Build map of UID -> is_unread
    let mut server_emails: Vec<(u32, bool)> = Vec::new();
    for msg in messages.iter() {
        if let Some(uid) = msg.uid {
            let flags = msg.flags();
            let is_unread = !flags.iter().any(|f| matches!(f, imap::types::Flag::Seen));
            server_emails.push((uid, is_unread));
        }
    }

    // Get cached UIDs
    let cache_dir = config_dir().join("cache").join(account_name).join(mailbox);
    fs::create_dir_all(&cache_dir)?;

    let mut cached_uids: HashSet<u32> = HashSet::new();
    if cache_dir.exists() {
        for entry in fs::read_dir(&cache_dir)? {
            let entry = entry?;
            let path = entry.path();
            if path.extension().map_or(false, |ext| ext == "json") {
                if let Some(stem) = path.file_stem() {
                    if let Ok(uid) = stem.to_string_lossy().parse::<u32>() {
                        cached_uids.insert(uid);
                    }
                }
            }
        }
    }

    // Find new UIDs to fetch
    let new_uids: Vec<u32> = server_emails
        .iter()
        .filter(|(uid, _)| !cached_uids.contains(uid))
        .map(|(uid, _)| *uid)
        .collect();

    eprintln!("[sync] Found {} new emails to download, {} cached", new_uids.len(), cached_uids.len());

    let mut new_count = 0;
    let mut updated_count = 0;

    // Fetch new emails in batches
    if !new_uids.is_empty() {
        let total_batches = (new_uids.len() + 49) / 50;
        for (batch_idx, chunk) in new_uids.chunks(50).enumerate() {
            eprintln!("[sync] Downloading batch {}/{} ({} emails)...", batch_idx + 1, total_batches, chunk.len());
            let uid_set: String = chunk
                .iter()
                .map(|u| u.to_string())
                .collect::<Vec<_>>()
                .join(",");

            let fetched = session.uid_fetch(&uid_set, "(UID FLAGS INTERNALDATE RFC822)")?;

            for msg in fetched.iter() {
                if let Some(uid) = msg.uid {
                    let flags = msg.flags();
                    let is_unread = !flags.iter().any(|f| matches!(f, imap::types::Flag::Seen));

                    let internal_date = msg
                        .internal_date()
                        .map(|d| d.to_rfc3339())
                        .unwrap_or_default();

                    if let Some(body) = msg.body() {
                        if let Ok(email) = parse_email_body(uid, body, is_unread, &internal_date) {
                            let cache_path = cache_dir.join(format!("{}.json", uid));
                            let json = serde_json::to_string_pretty(&email)?;
                            fs::write(&cache_path, json)?;
                            new_count += 1;
                        }
                    }
                }
            }
        }
    }

    // Update flags for existing emails
    for (uid, is_unread) in &server_emails {
        if cached_uids.contains(uid) {
            let cache_path = cache_dir.join(format!("{}.json", uid));
            if cache_path.exists() {
                let contents = fs::read_to_string(&cache_path)?;
                if let Ok(mut email) = serde_json::from_str::<Email>(&contents) {
                    if email.unread != *is_unread {
                        email.unread = *is_unread;
                        let json = serde_json::to_string_pretty(&email)?;
                        fs::write(&cache_path, json)?;
                        updated_count += 1;
                    }
                }
            }
        }
    }

    session.logout()?;

    eprintln!("[sync] Done! {} new, {} updated, {} total", new_count, updated_count, server_emails.len());

    Ok(SyncResult {
        new_emails: new_count,
        updated_emails: updated_count,
        total_emails: server_emails.len(),
    })
}

/// Sync emails from the last N days only (uses IMAP SINCE search)
pub fn sync_emails_since(
    account_name: &str,
    mailbox: &str,
    days: u32,
) -> Result<SyncResult, Box<dyn std::error::Error + Send + Sync>> {
    eprintln!("[sync] Starting sync for {} / {} (last {} days)", account_name, mailbox, days);

    let account = get_account(account_name).map_err(|e| e.to_string())?;
    eprintln!("[sync] Connecting to IMAP...");
    let mut session = connect_imap(&account).map_err(|e| e.to_string())?;
    eprintln!("[sync] Connected!");

    // Select mailbox
    eprintln!("[sync] Selecting mailbox...");
    session.select(mailbox).map_err(|e| e.to_string())?;

    // Calculate date for SINCE search (N days ago)
    let since_date = chrono::Utc::now() - chrono::Duration::days(days as i64);
    let since_str = since_date.format("%d-%b-%Y").to_string(); // e.g., "15-Oct-2024"
    eprintln!("[sync] Searching for emails since {}", since_str);

    // Search for UIDs since date
    let search_query = format!("SINCE {}", since_str);
    let uids = session.uid_search(&search_query).map_err(|e| e.to_string())?;
    let uid_list: Vec<u32> = uids.into_iter().collect();
    eprintln!("[sync] Found {} emails in date range", uid_list.len());

    if uid_list.is_empty() {
        session.logout().map_err(|e| e.to_string())?;
        return Ok(SyncResult {
            new_emails: 0,
            updated_emails: 0,
            total_emails: 0,
        });
    }

    // Fetch flags for these UIDs
    let uid_set: String = uid_list.iter().map(|u| u.to_string()).collect::<Vec<_>>().join(",");
    let messages = session.uid_fetch(&uid_set, "(UID FLAGS)").map_err(|e| e.to_string())?;

    // Build map of UID -> is_unread
    let mut server_emails: Vec<(u32, bool)> = Vec::new();
    for msg in messages.iter() {
        if let Some(uid) = msg.uid {
            let flags = msg.flags();
            let is_unread = !flags.iter().any(|f| matches!(f, imap::types::Flag::Seen));
            server_emails.push((uid, is_unread));
        }
    }
    eprintln!("[sync] Got flags for {} emails", server_emails.len());

    // Get cached UIDs
    let cache_dir = config_dir().join("cache").join(account_name).join(mailbox);
    fs::create_dir_all(&cache_dir).map_err(|e| e.to_string())?;

    let mut cached_uids: HashSet<u32> = HashSet::new();
    if cache_dir.exists() {
        for entry in fs::read_dir(&cache_dir).map_err(|e| e.to_string())? {
            let entry = entry.map_err(|e| e.to_string())?;
            let path = entry.path();
            if path.extension().map_or(false, |ext| ext == "json") {
                if let Some(stem) = path.file_stem() {
                    if let Ok(uid) = stem.to_string_lossy().parse::<u32>() {
                        cached_uids.insert(uid);
                    }
                }
            }
        }
    }

    // Find new UIDs to fetch
    let new_uids: Vec<u32> = server_emails
        .iter()
        .filter(|(uid, _)| !cached_uids.contains(uid))
        .map(|(uid, _)| *uid)
        .collect();

    eprintln!("[sync] Found {} new emails to download, {} already cached", new_uids.len(), cached_uids.len());

    let mut new_count = 0;
    let mut updated_count = 0;

    // Fetch new emails in batches
    if !new_uids.is_empty() {
        let total_batches = (new_uids.len() + 49) / 50;
        for (batch_idx, chunk) in new_uids.chunks(50).enumerate() {
            eprintln!("[sync] Downloading batch {}/{} ({} emails)...", batch_idx + 1, total_batches, chunk.len());
            let batch_uid_set: String = chunk
                .iter()
                .map(|u| u.to_string())
                .collect::<Vec<_>>()
                .join(",");

            let fetched = session
                .uid_fetch(&batch_uid_set, "(UID FLAGS INTERNALDATE RFC822)")
                .map_err(|e| e.to_string())?;

            for msg in fetched.iter() {
                if let Some(uid) = msg.uid {
                    let flags = msg.flags();
                    let is_unread = !flags.iter().any(|f| matches!(f, imap::types::Flag::Seen));

                    let internal_date = msg
                        .internal_date()
                        .map(|d| d.to_rfc3339())
                        .unwrap_or_default();

                    if let Some(body) = msg.body() {
                        if let Ok(email) = parse_email_body(uid, body, is_unread, &internal_date) {
                            let cache_path = cache_dir.join(format!("{}.json", uid));
                            let json = serde_json::to_string_pretty(&email).map_err(|e| e.to_string())?;
                            fs::write(&cache_path, json).map_err(|e| e.to_string())?;
                            new_count += 1;
                        }
                    }
                }
            }
        }
    }

    // Update flags for existing emails (only those in the date range)
    for (uid, is_unread) in &server_emails {
        if cached_uids.contains(uid) {
            let cache_path = cache_dir.join(format!("{}.json", uid));
            if cache_path.exists() {
                let contents = fs::read_to_string(&cache_path).map_err(|e| e.to_string())?;
                if let Ok(mut email) = serde_json::from_str::<Email>(&contents) {
                    if email.unread != *is_unread {
                        email.unread = *is_unread;
                        let json = serde_json::to_string_pretty(&email).map_err(|e| e.to_string())?;
                        fs::write(&cache_path, json).map_err(|e| e.to_string())?;
                        updated_count += 1;
                    }
                }
            }
        }
    }

    session.logout().map_err(|e| e.to_string())?;

    eprintln!("[sync] Done! {} new, {} updated, {} total", new_count, updated_count, server_emails.len());

    Ok(SyncResult {
        new_emails: new_count,
        updated_emails: updated_count,
        total_emails: server_emails.len(),
    })
}

fn parse_email_body(uid: u32, body: &[u8], is_unread: bool, internal_date: &str) -> Result<Email, Box<dyn std::error::Error>> {
    let parsed = parse_mail(body)?;

    let headers = &parsed.headers;

    let message_id = headers
        .get_first_value("Message-ID")
        .unwrap_or_default();
    let from = headers
        .get_first_value("From")
        .unwrap_or_default();
    let reply_to = headers
        .get_first_value("Reply-To")
        .unwrap_or_default();
    let to = headers
        .get_first_value("To")
        .unwrap_or_default();
    let cc = headers
        .get_first_value("Cc")
        .unwrap_or_default();
    let subject = headers
        .get_first_value("Subject")
        .unwrap_or_default();
    let date = headers
        .get_first_value("Date")
        .unwrap_or_default();

    // Extract body (HTML preferred, then plain text)
    let (body_html, snippet) = extract_body(&parsed);

    // Extract attachments
    let attachments = extract_attachments(&parsed);

    Ok(Email {
        uid,
        message_id,
        internal_date: internal_date.to_string(),
        from,
        reply_to,
        to,
        cc,
        subject,
        date,
        snippet,
        body_html,
        unread: is_unread,
        attachments,
    })
}

fn extract_body(parsed: &mailparse::ParsedMail) -> (String, String) {
    let mut html_body = String::new();
    let mut text_body = String::new();

    extract_body_recursive(parsed, &mut html_body, &mut text_body);

    // If we have HTML, clean it and use it
    if !html_body.is_empty() {
        let cleaned = ammonia::clean(&html_body);
        let snippet = generate_snippet(&text_body, &html_body);
        return (cleaned, snippet);
    }

    // Otherwise, convert plain text to simple HTML
    if !text_body.is_empty() {
        let snippet = text_body.chars().take(200).collect::<String>();
        let html = format!("<pre>{}</pre>", ammonia::clean(&text_body));
        return (html, snippet);
    }

    (String::new(), String::new())
}

fn extract_body_recursive(parsed: &mailparse::ParsedMail, html_body: &mut String, text_body: &mut String) {
    let content_type = parsed.ctype.mimetype.to_lowercase();

    if content_type == "text/html" && html_body.is_empty() {
        if let Ok(body) = parsed.get_body() {
            *html_body = body;
        }
    } else if content_type == "text/plain" && text_body.is_empty() {
        if let Ok(body) = parsed.get_body() {
            *text_body = body;
        }
    }

    // Recurse into subparts
    for subpart in &parsed.subparts {
        extract_body_recursive(subpart, html_body, text_body);
    }
}

fn generate_snippet(text_body: &str, html_body: &str) -> String {
    // Prefer text body for snippet
    if !text_body.is_empty() {
        return text_body
            .chars()
            .take(200)
            .collect::<String>()
            .replace('\n', " ")
            .replace('\r', "")
            .trim()
            .to_string();
    }

    // Fall back to stripping HTML
    if !html_body.is_empty() {
        let text = html2text::from_read(html_body.as_bytes(), 80);
        return text
            .chars()
            .take(200)
            .collect::<String>()
            .replace('\n', " ")
            .replace('\r', "")
            .trim()
            .to_string();
    }

    String::new()
}

fn extract_attachments(parsed: &mailparse::ParsedMail) -> Vec<Attachment> {
    let mut attachments = Vec::new();
    extract_attachments_recursive(parsed, &mut attachments, "");
    attachments
}

fn extract_attachments_recursive(parsed: &mailparse::ParsedMail, attachments: &mut Vec<Attachment>, part_prefix: &str) {
    let content_disposition = parsed
        .headers
        .get_first_value("Content-Disposition")
        .unwrap_or_default()
        .to_lowercase();

    let is_attachment = content_disposition.starts_with("attachment")
        || (content_disposition.starts_with("inline") && !parsed.ctype.mimetype.starts_with("text/"));

    if is_attachment {
        let filename = parsed
            .ctype
            .params
            .get("name")
            .cloned()
            .or_else(|| {
                // Try to extract from Content-Disposition
                if let Some(start) = content_disposition.find("filename=") {
                    let rest = &content_disposition[start + 9..];
                    let end = rest.find(';').unwrap_or(rest.len());
                    Some(rest[..end].trim_matches('"').to_string())
                } else {
                    None
                }
            })
            .unwrap_or_else(|| "unnamed".to_string());

        let encoding = parsed
            .headers
            .get_first_value("Content-Transfer-Encoding")
            .unwrap_or_default();

        let size = parsed.raw_bytes.len() as i64;
        let part_id = if part_prefix.is_empty() {
            "1".to_string()
        } else {
            format!("{}.{}", part_prefix, attachments.len() + 1)
        };

        attachments.push(Attachment {
            part_id,
            filename,
            content_type: parsed.ctype.mimetype.clone(),
            size,
            encoding,
        });
    }

    // Recurse into subparts
    for (i, subpart) in parsed.subparts.iter().enumerate() {
        let new_prefix = if part_prefix.is_empty() {
            format!("{}", i + 1)
        } else {
            format!("{}.{}", part_prefix, i + 1)
        };
        extract_attachments_recursive(subpart, attachments, &new_prefix);
    }
}
