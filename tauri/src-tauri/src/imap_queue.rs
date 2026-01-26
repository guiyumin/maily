use std::collections::HashMap;
use std::net::TcpStream;
use std::sync::{Arc, Mutex};
use std::time::{Duration, Instant};

use imap::Session;
use native_tls::TlsStream;
use once_cell::sync::Lazy;
use serde::Serialize;
use tauri::{AppHandle, Emitter};
use tokio::sync::mpsc::{self, Sender};

use crate::mail::{
    delete_email_from_cache, get_accounts, log_op, sync_emails_with_session, Account,
};

// ============ CONNECTION POOL ============

/// Connection staleness timeout (5 minutes)
const CONNECTION_TIMEOUT_SECS: u64 = 300;

/// Pooled IMAP connection for an account
struct PooledConnection {
    session: Option<Session<TlsStream<TcpStream>>>,
    last_used: Instant,
}

/// Per-account connection pool
static CONNECTION_POOL: Lazy<Mutex<HashMap<String, Arc<Mutex<PooledConnection>>>>> =
    Lazy::new(|| Mutex::new(HashMap::new()));

/// Get or create a pooled connection entry for an account
fn get_connection_pool(account_name: &str) -> Arc<Mutex<PooledConnection>> {
    let mut pool = CONNECTION_POOL.lock().unwrap();

    if let Some(conn) = pool.get(account_name) {
        return conn.clone();
    }

    let conn = Arc::new(Mutex::new(PooledConnection {
        session: None,
        last_used: Instant::now(),
    }));

    pool.insert(account_name.to_string(), conn.clone());
    conn
}

/// Check if an error indicates a broken connection
fn is_connection_error(err: &str) -> bool {
    let err_lower = err.to_lowercase();
    err_lower.contains("closed") ||
    err_lower.contains("reset") ||
    err_lower.contains("broken pipe") ||
    err_lower.contains("eof") ||
    err_lower.contains("connection") ||
    err_lower.contains("timed out") ||
    err_lower.contains("bye")
}

/// Create a new IMAP connection for an account
fn connect_imap_for_account(account: &Account) -> Result<Session<TlsStream<TcpStream>>, Box<dyn std::error::Error + Send + Sync>> {
    use std::net::ToSocketAddrs;

    let creds = &account.credentials;

    let addr = format!("{}:{}", creds.imap_host, creds.imap_port);
    let socket_addr = addr
        .to_socket_addrs()?
        .next()
        .ok_or("Failed to resolve IMAP host")?;

    let tcp = TcpStream::connect_timeout(&socket_addr, Duration::from_secs(30))?;
    tcp.set_read_timeout(Some(Duration::from_secs(60)))?;
    tcp.set_write_timeout(Some(Duration::from_secs(30)))?;

    let tls = native_tls::TlsConnector::builder().build()?;
    let tls_stream = tls.connect(&creds.imap_host, tcp)?;

    let client = imap::Client::new(tls_stream);
    let session = client.login(&creds.email, &creds.password).map_err(|e| e.0.to_string())?;

    eprintln!("[pool] Created new IMAP connection for {}", account.name);
    Ok(session)
}

/// Execute an operation with a pooled IMAP connection.
/// Handles reconnection on stale/failed connections.
/// This is the Rust equivalent of Go's `withIMAPClient()`.
fn with_imap_connection<F, R>(
    account_name: &str,
    mailbox: &str,
    op: F,
) -> Result<R, Box<dyn std::error::Error + Send + Sync>>
where
    F: FnOnce(&mut Session<TlsStream<TcpStream>>) -> Result<R, Box<dyn std::error::Error + Send + Sync>>,
{
    // Get account credentials
    let accounts = get_accounts().map_err(|e| e.to_string())?;
    let account = accounts
        .into_iter()
        .find(|a| a.name == account_name)
        .ok_or_else(|| format!("Account '{}' not found", account_name))?;

    let pool = get_connection_pool(account_name);
    let mut conn = pool.lock().unwrap();

    // Check if connection is stale or missing
    let is_stale = conn.session.is_some() &&
        conn.last_used.elapsed().as_secs() > CONNECTION_TIMEOUT_SECS;

    if conn.session.is_none() || is_stale {
        // Close stale connection
        if let Some(mut session) = conn.session.take() {
            eprintln!("[pool] Closing stale connection for {} (idle {}s)",
                account_name, conn.last_used.elapsed().as_secs());
            let _ = session.logout();
        }

        // Create new connection
        conn.session = Some(connect_imap_for_account(&account)?);
    } else {
        eprintln!("[pool] Reusing connection for {} (idle {}s)",
            account_name, conn.last_used.elapsed().as_secs());
    }

    let session = conn.session.as_mut().unwrap();

    // Select mailbox
    if let Err(e) = session.select(mailbox) {
        // Selection failed, try to reconnect
        eprintln!("[pool] Mailbox select failed, reconnecting: {}", e);
        conn.session = None;
        conn.session = Some(connect_imap_for_account(&account)?);
        let session = conn.session.as_mut().unwrap();
        session.select(mailbox)?;
    }

    // Execute operation
    let result = op(conn.session.as_mut().unwrap());

    // On connection error, invalidate pool entry
    if let Err(ref e) = result {
        if is_connection_error(&e.to_string()) {
            eprintln!("[pool] Connection error detected, invalidating pool: {}", e);
            if let Some(mut session) = conn.session.take() {
                let _ = session.logout();
            }
        }
    }

    conn.last_used = Instant::now();
    result
}

/// Close all pooled connections (for cleanup)
#[allow(dead_code)]
pub fn close_all_connections() {
    let mut pool = CONNECTION_POOL.lock().unwrap();
    for (name, conn) in pool.drain() {
        if let Ok(mut guard) = conn.lock() {
            if let Some(mut session) = guard.session.take() {
                eprintln!("[pool] Closing connection for {}", name);
                let _ = session.logout();
            }
        }
    }
}

/// Operations that can be queued for an account
#[derive(Debug, Clone)]
pub enum ImapOperation {
    MarkRead {
        mailbox: String,
        uid: u32,
        unread: bool,
    },
    Delete {
        mailbox: String,
        uid: u32,
    },
    MoveToTrash {
        mailbox: String,
        uid: u32,
    },
    SyncMailbox {
        mailbox: String,
    },
}

/// Events emitted to frontend
#[derive(Debug, Clone, Serialize)]
pub struct SyncStartedEvent {
    pub account: String,
    pub mailbox: String,
}

#[derive(Debug, Clone, Serialize)]
pub struct SyncCompleteEvent {
    pub account: String,
    pub mailbox: String,
    pub new_emails: usize,
    pub updated_emails: usize,
    pub total_emails: usize,
    #[serde(default)]
    pub deleted_emails: usize,
}

#[derive(Debug, Clone, Serialize)]
pub struct SyncErrorEvent {
    pub account: String,
    pub mailbox: String,
    pub error: String,
}

/// Per-account queue senders
static ACCOUNT_QUEUES: Lazy<Mutex<HashMap<String, Sender<ImapOperation>>>> =
    Lazy::new(|| Mutex::new(HashMap::new()));

/// App handle for emitting events
static APP_HANDLE: Lazy<Mutex<Option<AppHandle>>> = Lazy::new(|| Mutex::new(None));

/// Initialize with app handle (call from lib.rs setup)
pub fn init(app: AppHandle) {
    let mut handle = APP_HANDLE.lock().unwrap();
    *handle = Some(app);
}

/// Get or create sender for an account
fn get_account_sender(account_name: &str) -> Sender<ImapOperation> {
    let mut queues = ACCOUNT_QUEUES.lock().unwrap();

    if let Some(sender) = queues.get(account_name) {
        return sender.clone();
    }

    // Create new channel and spawn worker for this account
    let (sender, receiver) = mpsc::channel::<ImapOperation>(100);
    let account_name_owned = account_name.to_string();

    // Spawn tokio task for this account's worker
    tokio::spawn(async move {
        account_worker(account_name_owned, receiver).await;
    });

    queues.insert(account_name.to_string(), sender.clone());
    sender
}

/// Worker task for a single account - processes operations sequentially
async fn account_worker(account_name: String, mut receiver: mpsc::Receiver<ImapOperation>) {
    // Batch pending operations
    let mut pending_ops: HashMap<String, ImapOperation> = HashMap::new();

    loop {
        // Try to receive with timeout for batching
        match tokio::time::timeout(Duration::from_millis(100), receiver.recv()).await {
            Ok(Some(op)) => {
                match &op {
                    ImapOperation::MarkRead { mailbox, uid, .. } => {
                        let key = format!("mark:{}:{}", mailbox, uid);
                        pending_ops.insert(key, op);
                    }
                    ImapOperation::Delete { mailbox, uid } => {
                        let key = format!("del:{}:{}", mailbox, uid);
                        pending_ops.insert(key, op);
                    }
                    ImapOperation::MoveToTrash { mailbox, uid } => {
                        let key = format!("trash:{}:{}", mailbox, uid);
                        pending_ops.insert(key, op);
                    }
                    ImapOperation::SyncMailbox { mailbox } => {
                        // Process any pending ops first
                        if !pending_ops.is_empty() {
                            process_pending_ops(&account_name, &mut pending_ops).await;
                        }
                        // Then sync
                        process_sync(&account_name, mailbox).await;
                    }
                }
            }
            Ok(None) => {
                // Channel closed, exit
                break;
            }
            Err(_) => {
                // Timeout - process pending ops
                if !pending_ops.is_empty() {
                    process_pending_ops(&account_name, &mut pending_ops).await;
                }
            }
        }
    }
}

/// Process batched operations
async fn process_pending_ops(
    account_name: &str,
    pending: &mut HashMap<String, ImapOperation>,
) {
    let ops: Vec<_> = pending.drain().collect();

    for (_, op) in ops {
        let account = account_name.to_string();
        match op {
            ImapOperation::MarkRead { mailbox, uid, unread } => {
                let mbox = mailbox.clone();
                let result = tokio::task::spawn_blocking(move || {
                    mark_read_imap(&account, &mbox, uid, unread)
                })
                .await;

                if let Err(e) = result {
                    eprintln!("[imap] spawn_blocking error: {}", e);
                } else if let Ok(Err(e)) = result {
                    eprintln!("[imap] mark_read failed {}:{}/{}: {}", account_name, mailbox, uid, e);
                }
            }
            ImapOperation::Delete { mailbox, uid } => {
                let mbox = mailbox.clone();
                let result = tokio::task::spawn_blocking(move || {
                    delete_imap(&account, &mbox, uid)
                })
                .await;

                if let Err(e) = result {
                    eprintln!("[imap] spawn_blocking error: {}", e);
                    let _ = log_op(account_name, &mailbox, "delete", uid, "failed", &e.to_string());
                } else if let Ok(Err(e)) = result {
                    eprintln!("[imap] delete failed {}:{}/{}: {}", account_name, mailbox, uid, e);
                    let _ = log_op(account_name, &mailbox, "delete", uid, "failed", &e.to_string());
                } else {
                    // Delete from cache again in case sync pulled email back
                    let _ = delete_email_from_cache(account_name, &mailbox, uid);
                    let _ = log_op(account_name, &mailbox, "delete", uid, "success", "");
                }
            }
            ImapOperation::MoveToTrash { mailbox, uid } => {
                let mbox = mailbox.clone();
                let result = tokio::task::spawn_blocking(move || {
                    move_to_trash_imap(&account, &mbox, uid)
                })
                .await;

                if let Err(e) = result {
                    eprintln!("[imap] spawn_blocking error: {}", e);
                    let _ = log_op(account_name, &mailbox, "move_trash", uid, "failed", &e.to_string());
                } else if let Ok(Err(e)) = result {
                    eprintln!("[imap] move_to_trash failed {}:{}/{}: {}", account_name, mailbox, uid, e);
                    let _ = log_op(account_name, &mailbox, "move_trash", uid, "failed", &e.to_string());
                } else {
                    // Delete from cache again in case sync pulled email back
                    let _ = delete_email_from_cache(account_name, &mailbox, uid);
                    let _ = log_op(account_name, &mailbox, "move_trash", uid, "success", "");
                }
            }
            ImapOperation::SyncMailbox { .. } => {
                // Handled separately
            }
        }
    }
}

/// Sync using pooled connection
fn sync_with_pool(
    account_name: &str,
    mailbox: &str,
) -> Result<crate::mail::SyncResult, Box<dyn std::error::Error + Send + Sync>> {
    // Get account credentials
    let accounts = get_accounts().map_err(|e| e.to_string())?;
    let account = accounts
        .into_iter()
        .find(|a| a.name == account_name)
        .ok_or_else(|| format!("Account '{}' not found", account_name))?;

    let pool = get_connection_pool(account_name);
    let mut conn = pool.lock().unwrap();

    // Check if connection is stale or missing
    let is_stale = conn.session.is_some() &&
        conn.last_used.elapsed().as_secs() > CONNECTION_TIMEOUT_SECS;

    if conn.session.is_none() || is_stale {
        if let Some(mut session) = conn.session.take() {
            eprintln!("[pool] Closing stale connection for sync {} (idle {}s)",
                account_name, conn.last_used.elapsed().as_secs());
            let _ = session.logout();
        }
        conn.session = Some(connect_imap_for_account(&account)?);
    } else {
        eprintln!("[pool] Reusing connection for sync {} (idle {}s)",
            account_name, conn.last_used.elapsed().as_secs());
    }

    let session = conn.session.as_mut().unwrap();
    let result = sync_emails_with_session(session, account_name, mailbox);

    // On connection error, invalidate pool
    if let Err(ref e) = result {
        if is_connection_error(&e.to_string()) {
            eprintln!("[pool] Connection error during sync, invalidating: {}", e);
            if let Some(mut session) = conn.session.take() {
                let _ = session.logout();
            }
        }
    }

    conn.last_used = Instant::now();
    result
}

/// Process sync operation using pooled connection
async fn process_sync(account_name: &str, mailbox: &str) {
    let app = APP_HANDLE.lock().unwrap().clone();

    // Emit started event
    if let Some(ref app) = app {
        let _ = app.emit("sync-started", SyncStartedEvent {
            account: account_name.to_string(),
            mailbox: mailbox.to_string(),
        });
    }

    eprintln!("[sync] Starting sync for {} / {}", account_name, mailbox);

    let account = account_name.to_string();
    let mbox = mailbox.to_string();

    // Run blocking IMAP sync on thread pool using pooled connection
    let result = tokio::task::spawn_blocking(move || {
        sync_with_pool(&account, &mbox)
    })
    .await;

    match result {
        Ok(Ok(sync_result)) => {
            eprintln!(
                "[sync] Done! {} new, {} updated, {} deleted, {} total",
                sync_result.new_emails, sync_result.updated_emails, sync_result.deleted_emails, sync_result.total_emails
            );

            if let Some(ref app) = app {
                let _ = app.emit("sync-complete", SyncCompleteEvent {
                    account: account_name.to_string(),
                    mailbox: mailbox.to_string(),
                    new_emails: sync_result.new_emails,
                    updated_emails: sync_result.updated_emails,
                    total_emails: sync_result.total_emails,
                    deleted_emails: sync_result.deleted_emails,
                });
            }
        }
        Ok(Err(e)) => {
            eprintln!("[sync] Error: {}", e);

            if let Some(ref app) = app {
                let _ = app.emit("sync-error", SyncErrorEvent {
                    account: account_name.to_string(),
                    mailbox: mailbox.to_string(),
                    error: e.to_string(),
                });
            }
        }
        Err(e) => {
            eprintln!("[sync] spawn_blocking error: {}", e);

            if let Some(ref app) = app {
                let _ = app.emit("sync-error", SyncErrorEvent {
                    account: account_name.to_string(),
                    mailbox: mailbox.to_string(),
                    error: format!("Internal error: {}", e),
                });
            }
        }
    }
}

/// IMAP mark read/unread using pooled connection
fn mark_read_imap(
    account_name: &str,
    mailbox: &str,
    uid: u32,
    unread: bool,
) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
    with_imap_connection(account_name, mailbox, |session| {
        let uid_set = format!("{}", uid);
        if unread {
            session.uid_store(&uid_set, "-FLAGS (\\Seen)")?;
        } else {
            session.uid_store(&uid_set, "+FLAGS (\\Seen)")?;
        }
        Ok(())
    })
}

/// Queue mark read/unread - returns immediately
pub fn queue_mark_read(account: String, mailbox: String, uid: u32, unread: bool) {
    let sender = get_account_sender(&account);
    let _ = sender.try_send(ImapOperation::MarkRead { mailbox, uid, unread });
}

/// Queue permanent delete - returns immediately
pub fn queue_delete(account: String, mailbox: String, uid: u32) {
    let sender = get_account_sender(&account);
    let _ = sender.try_send(ImapOperation::Delete { mailbox, uid });
}

/// Queue move to trash - returns immediately
pub fn queue_move_to_trash(account: String, mailbox: String, uid: u32) {
    let sender = get_account_sender(&account);
    let _ = sender.try_send(ImapOperation::MoveToTrash { mailbox, uid });
}

/// Queue sync - returns immediately
pub fn queue_sync(account: String, mailbox: String) {
    let sender = get_account_sender(&account);
    let _ = sender.try_send(ImapOperation::SyncMailbox { mailbox });
}

/// Fetch email body via pooled connection (for lazy-load)
pub fn fetch_body_via_pool(
    account_name: &str,
    mailbox: &str,
    uid: u32,
) -> Result<Option<(String, String)>, Box<dyn std::error::Error + Send + Sync>> {
    with_imap_connection(account_name, mailbox, |session| {
        let uid_str = uid.to_string();
        let fetched = session.uid_fetch(&uid_str, "(UID RFC822)")?;

        for msg in fetched.iter() {
            if msg.uid == Some(uid) {
                if let Some(body_bytes) = msg.body() {
                    if let Ok(parsed) = mailparse::parse_mail(body_bytes) {
                        let (body_html, snippet) = crate::mail::extract_body(&parsed);
                        return Ok(Some((body_html, snippet)));
                    }
                }
            }
        }

        Ok(None)
    })
}

/// IMAP delete using pooled connection
fn delete_imap(
    account_name: &str,
    mailbox: &str,
    uid: u32,
) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
    with_imap_connection(account_name, mailbox, |session| {
        // Mark as deleted and expunge
        let uid_set = format!("{}", uid);
        session.uid_store(&uid_set, "+FLAGS (\\Deleted)")?;
        session.expunge()?;
        Ok(())
    })
}

/// IMAP move to trash using pooled connection
fn move_to_trash_imap(
    account_name: &str,
    mailbox: &str,
    uid: u32,
) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
    // Get account to determine provider
    let accounts = get_accounts().map_err(|e| e.to_string())?;
    let account = accounts
        .into_iter()
        .find(|a| a.name == account_name)
        .ok_or_else(|| format!("Account '{}' not found", account_name))?;

    // Determine trash folder based on provider
    let trash_folder = if account.credentials.imap_host.contains("gmail") {
        "[Gmail]/Trash"
    } else if account.credentials.imap_host.contains("yahoo") {
        "Trash"
    } else {
        "Trash"
    };

    with_imap_connection(account_name, mailbox, |session| {
        // Move to trash using COPY + DELETE
        let uid_set = format!("{}", uid);
        session.uid_copy(&uid_set, trash_folder)?;
        session.uid_store(&uid_set, "+FLAGS (\\Deleted)")?;
        session.expunge()?;
        Ok(())
    })
}
