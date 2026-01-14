use imap::Session;
use mailparse::{parse_mail, MailHeaderMap};
use native_tls::TlsStream;
use rusqlite::{params, Connection};
use serde::{Deserialize, Serialize};
use std::collections::HashSet;
use std::fs;
use std::net::TcpStream;
use std::path::PathBuf;
use std::sync::Mutex;
use std::time::Duration;
use once_cell::sync::Lazy;

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

/// Result of paginated email list
#[derive(Debug, Serialize, Clone)]
pub struct ListEmailsResult {
    pub emails: Vec<EmailSummary>,
    pub total: usize,
    pub offset: usize,
    pub has_more: bool,
}

/// Initial app state - everything needed to render on startup
#[derive(Debug, Serialize, Clone)]
pub struct InitialState {
    pub accounts: Vec<Account>,
    pub selected_account: Option<String>,
    pub emails: ListEmailsResult,
}

const SCHEMA: &str = r#"
CREATE TABLE IF NOT EXISTS mailbox_metadata (
    account TEXT NOT NULL,
    mailbox TEXT NOT NULL,
    uid_validity INTEGER NOT NULL DEFAULT 0,
    last_sync INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (account, mailbox)
);

CREATE TABLE IF NOT EXISTS emails (
    account TEXT NOT NULL,
    mailbox TEXT NOT NULL,
    uid INTEGER NOT NULL,
    message_id TEXT NOT NULL DEFAULT '',
    internal_date INTEGER NOT NULL,
    from_addr TEXT NOT NULL DEFAULT '',
    reply_to TEXT NOT NULL DEFAULT '',
    to_addr TEXT NOT NULL DEFAULT '',
    cc TEXT NOT NULL DEFAULT '',
    subject TEXT NOT NULL DEFAULT '',
    date TEXT NOT NULL DEFAULT '',
    snippet TEXT NOT NULL DEFAULT '',
    body_html TEXT NOT NULL DEFAULT '',
    unread INTEGER NOT NULL DEFAULT 1,
    references_hdr TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (account, mailbox, uid)
);

CREATE TABLE IF NOT EXISTS attachments (
    account TEXT NOT NULL,
    mailbox TEXT NOT NULL,
    email_uid INTEGER NOT NULL,
    part_id TEXT NOT NULL,
    filename TEXT NOT NULL DEFAULT '',
    content_type TEXT NOT NULL DEFAULT '',
    size INTEGER NOT NULL DEFAULT 0,
    encoding TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (account, mailbox, email_uid, part_id),
    FOREIGN KEY (account, mailbox, email_uid)
        REFERENCES emails(account, mailbox, uid) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS op_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    account TEXT NOT NULL,
    mailbox TEXT NOT NULL,
    operation TEXT NOT NULL,
    uid INTEGER NOT NULL,
    status TEXT NOT NULL,
    error TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    processed_at INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_emails_date ON emails(account, mailbox, internal_date DESC);
CREATE INDEX IF NOT EXISTS idx_emails_internal_date ON emails(internal_date);
CREATE INDEX IF NOT EXISTS idx_op_logs_account ON op_logs(account);
CREATE INDEX IF NOT EXISTS idx_op_logs_processed ON op_logs(processed_at DESC);
"#;

/// Initialize database eagerly (call on app startup)
pub fn init_db() {
    // Force lazy initialization by accessing DB
    drop(DB.lock());
}

// Thread-safe database connection
static DB: Lazy<Mutex<Connection>> = Lazy::new(|| {
    let db_path = config_dir().join("maily.db");
    let conn = Connection::open(&db_path).expect("Failed to open database");

    // Enable WAL mode, foreign keys
    conn.execute_batch("
        PRAGMA journal_mode=WAL;
        PRAGMA foreign_keys=ON;
        PRAGMA busy_timeout=5000;
    ").expect("Failed to set pragmas");

    // Create schema
    conn.execute_batch(SCHEMA).expect("Failed to create schema");

    // Clean up old JSON cache directory
    let old_cache_dir = config_dir().join("cache");
    if old_cache_dir.exists() {
        let _ = fs::remove_dir_all(&old_cache_dir);
    }

    Mutex::new(conn)
});

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

/// Get everything needed to render the app on startup in ONE call
pub fn get_initial_state() -> Result<InitialState, Box<dyn std::error::Error>> {
    let accounts = get_accounts().unwrap_or_default();

    let (selected_account, emails) = if let Some(first) = accounts.first() {
        let result = list_emails_paginated(&first.name, "INBOX", 0, 50)?;
        (Some(first.name.clone()), result)
    } else {
        (None, ListEmailsResult {
            emails: vec![],
            total: 0,
            offset: 0,
            has_more: false,
        })
    };

    Ok(InitialState {
        accounts,
        selected_account,
        emails,
    })
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

fn load_attachments(conn: &Connection, account: &str, mailbox: &str, uid: u32) -> Vec<Attachment> {
    let mut stmt = match conn.prepare(
        "SELECT part_id, filename, content_type, size, encoding FROM attachments WHERE account = ?1 AND mailbox = ?2 AND email_uid = ?3"
    ) {
        Ok(s) => s,
        Err(_) => return vec![],
    };

    let attachments = stmt.query_map(params![account, mailbox, uid], |row| {
        Ok(Attachment {
            part_id: row.get(0)?,
            filename: row.get(1)?,
            content_type: row.get(2)?,
            size: row.get(3)?,
            encoding: row.get(4)?,
        })
    });

    match attachments {
        Ok(iter) => iter.filter_map(|r| r.ok()).collect(),
        Err(_) => vec![],
    }
}

pub fn get_emails(account: &str, mailbox: &str) -> Result<Vec<Email>, Box<dyn std::error::Error>> {
    let conn = DB.lock().unwrap();

    let mut stmt = conn.prepare(
        "SELECT uid, message_id, internal_date, from_addr, reply_to, to_addr, cc, subject, date, snippet, body_html, unread, references_hdr
         FROM emails WHERE account = ?1 AND mailbox = ?2
         ORDER BY internal_date DESC"
    )?;

    let emails: Vec<Email> = stmt.query_map(params![account, mailbox], |row| {
        let uid: u32 = row.get(0)?;
        let internal_date_ts: i64 = row.get(2)?;
        let unread: i32 = row.get(11)?;

        Ok(Email {
            uid,
            message_id: row.get(1)?,
            internal_date: chrono::DateTime::from_timestamp(internal_date_ts, 0)
                .map(|dt| dt.to_rfc3339())
                .unwrap_or_default(),
            from: row.get(3)?,
            reply_to: row.get(4)?,
            to: row.get(5)?,
            cc: row.get(6)?,
            subject: row.get(7)?,
            date: row.get(8)?,
            snippet: row.get(9)?,
            body_html: row.get(10)?,
            unread: unread == 1,
            attachments: vec![], // Will be loaded separately
        })
    })?.filter_map(|r| r.ok()).collect();

    // Load attachments for each email
    let emails_with_attachments: Vec<Email> = emails.into_iter().map(|mut email| {
        email.attachments = load_attachments(&conn, account, mailbox, email.uid);
        email
    }).collect();

    Ok(emails_with_attachments)
}

/// Get paginated email list (lightweight summaries)
pub fn list_emails_paginated(
    account: &str,
    mailbox: &str,
    offset: usize,
    limit: usize,
) -> Result<ListEmailsResult, Box<dyn std::error::Error>> {
    let conn = DB.lock().unwrap();

    // Get total count
    let total: usize = conn.query_row(
        "SELECT COUNT(*) FROM emails WHERE account = ?1 AND mailbox = ?2",
        params![account, mailbox],
        |row| row.get(0)
    )?;

    if total == 0 {
        return Ok(ListEmailsResult {
            emails: vec![],
            total: 0,
            offset,
            has_more: false,
        });
    }

    let has_more = offset + limit < total;

    // Get paginated results
    let mut stmt = conn.prepare(
        "SELECT uid, message_id, internal_date, from_addr, to_addr, subject, date, snippet, unread,
                EXISTS(SELECT 1 FROM attachments WHERE attachments.account = emails.account
                       AND attachments.mailbox = emails.mailbox AND attachments.email_uid = emails.uid) as has_attachments
         FROM emails WHERE account = ?1 AND mailbox = ?2
         ORDER BY internal_date DESC
         LIMIT ?3 OFFSET ?4"
    )?;

    let emails: Vec<EmailSummary> = stmt.query_map(
        params![account, mailbox, limit as i64, offset as i64],
        |row| {
            let internal_date_ts: i64 = row.get(2)?;
            let unread: i32 = row.get(8)?;
            let has_attachments: i32 = row.get(9)?;

            Ok(EmailSummary {
                uid: row.get(0)?,
                message_id: row.get(1)?,
                internal_date: chrono::DateTime::from_timestamp(internal_date_ts, 0)
                    .map(|dt| dt.to_rfc3339())
                    .unwrap_or_default(),
                from: row.get(3)?,
                to: row.get(4)?,
                subject: row.get(5)?,
                date: row.get(6)?,
                snippet: row.get(7)?,
                unread: unread == 1,
                has_attachments: has_attachments == 1,
            })
        }
    )?.filter_map(|r| r.ok()).collect();

    Ok(ListEmailsResult {
        emails,
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
    let conn = DB.lock().unwrap();
    let cutoff = chrono::Utc::now().timestamp() - (days * 86400);

    let count: usize = conn.query_row(
        "SELECT COUNT(*) FROM emails WHERE account = ?1 AND mailbox = ?2 AND internal_date >= ?3",
        params![account, mailbox, cutoff],
        |row| row.get(0)
    )?;

    Ok(count)
}

pub fn get_email(account: &str, mailbox: &str, uid: u32) -> Result<Email, Box<dyn std::error::Error>> {
    let conn = DB.lock().unwrap();

    let mut stmt = conn.prepare(
        "SELECT uid, message_id, internal_date, from_addr, reply_to, to_addr, cc, subject, date, snippet, body_html, unread, references_hdr
         FROM emails WHERE account = ?1 AND mailbox = ?2 AND uid = ?3"
    )?;

    let email = stmt.query_row(params![account, mailbox, uid], |row| {
        let internal_date_ts: i64 = row.get(2)?;
        let unread: i32 = row.get(11)?;

        Ok(Email {
            uid: row.get(0)?,
            message_id: row.get(1)?,
            internal_date: chrono::DateTime::from_timestamp(internal_date_ts, 0)
                .map(|dt| dt.to_rfc3339())
                .unwrap_or_default(),
            from: row.get(3)?,
            reply_to: row.get(4)?,
            to: row.get(5)?,
            cc: row.get(6)?,
            subject: row.get(7)?,
            date: row.get(8)?,
            snippet: row.get(9)?,
            body_html: row.get(10)?,
            unread: unread == 1,
            attachments: vec![],
        })
    })?;

    // Load attachments
    let mut email = email;
    email.attachments = load_attachments(&conn, account, mailbox, uid);

    Ok(email)
}

pub fn delete_email_from_cache(account: &str, mailbox: &str, uid: u32) -> Result<(), Box<dyn std::error::Error>> {
    let conn = DB.lock().unwrap();
    conn.execute(
        "DELETE FROM emails WHERE account = ?1 AND mailbox = ?2 AND uid = ?3",
        params![account, mailbox, uid]
    )?;
    Ok(())
}

pub fn log_op(account: &str, mailbox: &str, operation: &str, uid: u32, status: &str, error: &str) -> Result<(), Box<dyn std::error::Error>> {
    use std::time::{SystemTime, UNIX_EPOCH};
    let now = SystemTime::now().duration_since(UNIX_EPOCH)?.as_secs() as i64;
    let conn = DB.lock().unwrap();
    conn.execute(
        "INSERT INTO op_logs (account, mailbox, operation, uid, status, error, created_at, processed_at) VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?7)",
        params![account, mailbox, operation, uid, status, error, now]
    )?;
    Ok(())
}

pub fn update_email_read_status(account_name: &str, mailbox: &str, uid: u32, unread: bool) -> Result<Email, Box<dyn std::error::Error>> {
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
    {
        let conn = DB.lock().unwrap();
        let unread_val = if unread { 1 } else { 0 };
        conn.execute(
            "UPDATE emails SET unread = ?1 WHERE account = ?2 AND mailbox = ?3 AND uid = ?4",
            params![unread_val, account_name, mailbox, uid]
        )?;
    }

    // Return updated email
    get_email(account_name, mailbox, uid)
}

/// Update only the local cache (no IMAP) - used with background queue
pub fn update_email_cache_only(account_name: &str, mailbox: &str, uid: u32, unread: bool) -> Result<Email, Box<dyn std::error::Error>> {
    {
        let conn = DB.lock().unwrap();
        let unread_val = if unread { 1 } else { 0 };
        conn.execute(
            "UPDATE emails SET unread = ?1 WHERE account = ?2 AND mailbox = ?3 AND uid = ?4",
            params![unread_val, account_name, mailbox, uid]
        )?;
    }

    get_email(account_name, mailbox, uid)
}

fn save_email_to_db(conn: &Connection, account: &str, mailbox: &str, email: &Email) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
    let internal_date_ts = chrono::DateTime::parse_from_rfc3339(&email.internal_date)
        .map(|dt| dt.timestamp())
        .unwrap_or(0);

    let unread_val = if email.unread { 1 } else { 0 };

    conn.execute(
        "INSERT OR REPLACE INTO emails (account, mailbox, uid, message_id, internal_date, from_addr, reply_to, to_addr, cc, subject, date, snippet, body_html, unread, references_hdr)
         VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11, ?12, ?13, ?14, ?15)",
        params![
            account, mailbox, email.uid, email.message_id, internal_date_ts,
            email.from, email.reply_to, email.to, email.cc, email.subject,
            email.date, email.snippet, email.body_html, unread_val, ""
        ]
    ).map_err(|e| e.to_string())?;

    // Delete existing attachments and re-insert
    conn.execute(
        "DELETE FROM attachments WHERE account = ?1 AND mailbox = ?2 AND email_uid = ?3",
        params![account, mailbox, email.uid]
    ).map_err(|e| e.to_string())?;

    for att in &email.attachments {
        conn.execute(
            "INSERT INTO attachments (account, mailbox, email_uid, part_id, filename, content_type, size, encoding)
             VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8)",
            params![
                account, mailbox, email.uid, att.part_id, att.filename,
                att.content_type, att.size, att.encoding
            ]
        ).map_err(|e| e.to_string())?;
    }

    Ok(())
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

    // Get cached UIDs from SQLite
    let cached_uids: HashSet<u32> = {
        let conn = DB.lock().unwrap();
        let mut stmt = conn.prepare(
            "SELECT uid FROM emails WHERE account = ?1 AND mailbox = ?2"
        )?;
        let uids = stmt.query_map(params![account_name, mailbox], |row| {
            row.get::<_, u32>(0)
        })?;
        uids.filter_map(|r| r.ok()).collect()
    };

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

            let conn = DB.lock().unwrap();
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
                            if save_email_to_db(&conn, account_name, mailbox, &email).is_ok() {
                                new_count += 1;
                            }
                        }
                    }
                }
            }
        }
    }

    // Update flags for existing emails
    {
        let conn = DB.lock().unwrap();
        for (uid, is_unread) in &server_emails {
            if cached_uids.contains(uid) {
                // Check if flag changed
                let current_unread: Option<i32> = conn.query_row(
                    "SELECT unread FROM emails WHERE account = ?1 AND mailbox = ?2 AND uid = ?3",
                    params![account_name, mailbox, uid],
                    |row| row.get(0)
                ).ok();

                if let Some(current) = current_unread {
                    let new_unread = if *is_unread { 1 } else { 0 };
                    if current != new_unread {
                        conn.execute(
                            "UPDATE emails SET unread = ?1 WHERE account = ?2 AND mailbox = ?3 AND uid = ?4",
                            params![new_unread, account_name, mailbox, uid]
                        )?;
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

    // Get cached UIDs from SQLite
    let cached_uids: HashSet<u32> = {
        let conn = DB.lock().unwrap();
        let mut stmt = conn.prepare(
            "SELECT uid FROM emails WHERE account = ?1 AND mailbox = ?2"
        ).map_err(|e| e.to_string())?;
        let uids = stmt.query_map(params![account_name, mailbox], |row| {
            row.get::<_, u32>(0)
        }).map_err(|e| e.to_string())?;
        uids.filter_map(|r| r.ok()).collect()
    };

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

            let conn = DB.lock().unwrap();
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
                            if save_email_to_db(&conn, account_name, mailbox, &email).is_ok() {
                                new_count += 1;
                            }
                        }
                    }
                }
            }
        }
    }

    // Update flags for existing emails (only those in the date range)
    {
        let conn = DB.lock().unwrap();
        for (uid, is_unread) in &server_emails {
            if cached_uids.contains(uid) {
                let current_unread: Option<i32> = conn.query_row(
                    "SELECT unread FROM emails WHERE account = ?1 AND mailbox = ?2 AND uid = ?3",
                    params![account_name, mailbox, uid],
                    |row| row.get(0)
                ).ok();

                if let Some(current) = current_unread {
                    let new_unread = if *is_unread { 1 } else { 0 };
                    if current != new_unread {
                        let _ = conn.execute(
                            "UPDATE emails SET unread = ?1 WHERE account = ?2 AND mailbox = ?3 AND uid = ?4",
                            params![new_unread, account_name, mailbox, uid]
                        );
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
