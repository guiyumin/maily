use std::collections::HashMap;
use std::sync::Mutex;
use std::time::Duration;

use once_cell::sync::Lazy;
use serde::Serialize;
use tauri::{AppHandle, Emitter};
use tokio::sync::mpsc::{self, Sender};

use crate::mail::{delete_email_from_cache, get_accounts, sync_emails_since};

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
                } else if let Ok(Err(e)) = result {
                    eprintln!("[imap] delete failed {}:{}/{}: {}", account_name, mailbox, uid, e);
                } else {
                    // Delete from cache again in case sync pulled email back
                    let _ = delete_email_from_cache(account_name, &mailbox, uid);
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
                } else if let Ok(Err(e)) = result {
                    eprintln!("[imap] move_to_trash failed {}:{}/{}: {}", account_name, mailbox, uid, e);
                } else {
                    // Delete from cache again in case sync pulled email back
                    let _ = delete_email_from_cache(account_name, &mailbox, uid);
                }
            }
            ImapOperation::SyncMailbox { .. } => {
                // Handled separately
            }
        }
    }
}

/// Process sync operation
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

    // Run blocking IMAP sync on thread pool
    let result = tokio::task::spawn_blocking(move || {
        sync_emails_since(&account, &mbox, 90)
    })
    .await;

    match result {
        Ok(Ok(sync_result)) => {
            eprintln!(
                "[sync] Done! {} new, {} updated, {} total",
                sync_result.new_emails, sync_result.updated_emails, sync_result.total_emails
            );

            if let Some(ref app) = app {
                let _ = app.emit("sync-complete", SyncCompleteEvent {
                    account: account_name.to_string(),
                    mailbox: mailbox.to_string(),
                    new_emails: sync_result.new_emails,
                    updated_emails: sync_result.updated_emails,
                    total_emails: sync_result.total_emails,
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

/// IMAP mark read/unread (runs on blocking thread pool)
fn mark_read_imap(
    account_name: &str,
    mailbox: &str,
    uid: u32,
    unread: bool,
) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
    use std::net::{TcpStream, ToSocketAddrs};

    let accounts = get_accounts().map_err(|e| e.to_string())?;
    let account = accounts
        .into_iter()
        .find(|a| a.name == account_name)
        .ok_or_else(|| format!("Account '{}' not found", account_name))?;

    let creds = &account.credentials;

    // Resolve hostname and connect with timeout
    let addr = format!("{}:{}", creds.imap_host, creds.imap_port);
    let socket_addr = addr
        .to_socket_addrs()?
        .next()
        .ok_or("Failed to resolve IMAP host")?;

    let tcp = TcpStream::connect_timeout(&socket_addr, Duration::from_secs(30))?;
    tcp.set_read_timeout(Some(Duration::from_secs(30)))?;
    tcp.set_write_timeout(Some(Duration::from_secs(30)))?;

    let tls = native_tls::TlsConnector::builder().build()?;
    let tls_stream = tls.connect(&creds.imap_host, tcp)?;

    let client = imap::Client::new(tls_stream);
    let mut session = client.login(&creds.email, &creds.password).map_err(|e| e.0)?;

    session.select(mailbox)?;

    let uid_set = format!("{}", uid);
    if unread {
        session.uid_store(&uid_set, "-FLAGS (\\Seen)")?;
    } else {
        session.uid_store(&uid_set, "+FLAGS (\\Seen)")?;
    }

    session.logout()?;
    Ok(())
}

/// Queue mark read/unread - returns immediately
pub fn queue_mark_read(account: String, mailbox: String, uid: u32, unread: bool) {
    let sender = get_account_sender(&account);
    let _ = sender.try_send(ImapOperation::MarkRead { mailbox, uid, unread });
}

/// Queue delete - returns immediately
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

/// IMAP delete (runs on blocking thread pool)
fn delete_imap(
    account_name: &str,
    mailbox: &str,
    uid: u32,
) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
    use std::net::{TcpStream, ToSocketAddrs};

    let accounts = get_accounts().map_err(|e| e.to_string())?;
    let account = accounts
        .into_iter()
        .find(|a| a.name == account_name)
        .ok_or_else(|| format!("Account '{}' not found", account_name))?;

    let creds = &account.credentials;

    let addr = format!("{}:{}", creds.imap_host, creds.imap_port);
    let socket_addr = addr
        .to_socket_addrs()?
        .next()
        .ok_or("Failed to resolve IMAP host")?;

    let tcp = TcpStream::connect_timeout(&socket_addr, Duration::from_secs(30))?;
    tcp.set_read_timeout(Some(Duration::from_secs(30)))?;
    tcp.set_write_timeout(Some(Duration::from_secs(30)))?;

    let tls = native_tls::TlsConnector::builder().build()?;
    let tls_stream = tls.connect(&creds.imap_host, tcp)?;

    let client = imap::Client::new(tls_stream);
    let mut session = client.login(&creds.email, &creds.password).map_err(|e| e.0)?;

    session.select(mailbox)?;

    // Mark as deleted and expunge
    let uid_set = format!("{}", uid);
    session.uid_store(&uid_set, "+FLAGS (\\Deleted)")?;
    session.expunge()?;

    session.logout()?;
    Ok(())
}

/// IMAP move to trash (runs on blocking thread pool)
fn move_to_trash_imap(
    account_name: &str,
    mailbox: &str,
    uid: u32,
) -> Result<(), Box<dyn std::error::Error + Send + Sync>> {
    use std::net::{TcpStream, ToSocketAddrs};

    let accounts = get_accounts().map_err(|e| e.to_string())?;
    let account = accounts
        .into_iter()
        .find(|a| a.name == account_name)
        .ok_or_else(|| format!("Account '{}' not found", account_name))?;

    let creds = &account.credentials;

    let addr = format!("{}:{}", creds.imap_host, creds.imap_port);
    let socket_addr = addr
        .to_socket_addrs()?
        .next()
        .ok_or("Failed to resolve IMAP host")?;

    let tcp = TcpStream::connect_timeout(&socket_addr, Duration::from_secs(30))?;
    tcp.set_read_timeout(Some(Duration::from_secs(30)))?;
    tcp.set_write_timeout(Some(Duration::from_secs(30)))?;

    let tls = native_tls::TlsConnector::builder().build()?;
    let tls_stream = tls.connect(&creds.imap_host, tcp)?;

    let client = imap::Client::new(tls_stream);
    let mut session = client.login(&creds.email, &creds.password).map_err(|e| e.0)?;

    session.select(mailbox)?;

    // Determine trash folder based on provider
    let trash_folder = if creds.imap_host.contains("gmail") {
        "[Gmail]/Trash"
    } else if creds.imap_host.contains("yahoo") {
        "Trash"
    } else {
        "Trash"
    };

    // Move to trash using COPY + DELETE
    let uid_set = format!("{}", uid);
    session.uid_copy(&uid_set, trash_folder)?;
    session.uid_store(&uid_set, "+FLAGS (\\Deleted)")?;
    session.expunge()?;

    session.logout()?;
    Ok(())
}
