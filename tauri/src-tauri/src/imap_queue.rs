use std::collections::HashMap;
use std::sync::mpsc::{self, Receiver, Sender};
use std::sync::OnceLock;
use std::thread;

use crate::mail::get_accounts;

#[derive(Debug, Clone)]
pub enum ImapOperation {
    MarkRead {
        account: String,
        mailbox: String,
        uid: u32,
        unread: bool,
    },
}

static QUEUE_SENDER: OnceLock<Sender<ImapOperation>> = OnceLock::new();

fn get_sender() -> &'static Sender<ImapOperation> {
    QUEUE_SENDER.get_or_init(|| {
        let (sender, receiver) = mpsc::channel::<ImapOperation>();

        // Start background worker thread
        thread::spawn(move || {
            worker_loop(receiver);
        });

        sender
    })
}

fn worker_loop(receiver: Receiver<ImapOperation>) {
    // Batch operations by key to deduplicate rapid toggles
    let mut pending: HashMap<String, ImapOperation> = HashMap::new();

    loop {
        // Try to receive with a short timeout to allow batching
        match receiver.recv_timeout(std::time::Duration::from_millis(100)) {
            Ok(op) => {
                let key = match &op {
                    ImapOperation::MarkRead { account, mailbox, uid, .. } => {
                        format!("{}:{}:{}", account, mailbox, uid)
                    }
                };
                // Latest operation wins (overwrites previous)
                pending.insert(key, op);
            }
            Err(mpsc::RecvTimeoutError::Timeout) => {
                // Process all pending operations
                if !pending.is_empty() {
                    let ops: Vec<_> = pending.drain().collect();
                    for (_, op) in ops {
                        process_operation(op);
                    }
                }
            }
            Err(mpsc::RecvTimeoutError::Disconnected) => {
                // Channel closed, exit thread
                break;
            }
        }
    }
}

fn process_operation(op: ImapOperation) {
    match op {
        ImapOperation::MarkRead { account, mailbox, uid, unread } => {
            if let Err(e) = mark_read_imap(&account, &mailbox, uid, unread) {
                eprintln!("IMAP mark read failed for {}:{}/{}: {}", account, mailbox, uid, e);
                // Could implement retry logic here
            }
        }
    }
}

fn mark_read_imap(account_name: &str, mailbox: &str, uid: u32, unread: bool) -> Result<(), Box<dyn std::error::Error>> {
    let accounts = get_accounts()?;
    let account = accounts
        .into_iter()
        .find(|a| a.name == account_name)
        .ok_or_else(|| format!("Account '{}' not found", account_name))?;

    let creds = &account.credentials;
    let tls = native_tls::TlsConnector::builder().build()?;
    let client = imap::connect((creds.imap_host.as_str(), creds.imap_port), &creds.imap_host, &tls)?;
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

/// Queue an operation - returns immediately
pub fn queue_operation(op: ImapOperation) {
    let sender = get_sender();
    let _ = sender.send(op);
}

/// Queue mark read/unread - returns immediately
pub fn queue_mark_read(account: String, mailbox: String, uid: u32, unread: bool) {
    queue_operation(ImapOperation::MarkRead {
        account,
        mailbox,
        uid,
        unread,
    });
}
