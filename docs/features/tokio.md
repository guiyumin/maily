# Tokio Migration Plan

Migrate the Tauri Rust backend from std threads to tokio for async concurrency.

## Why

- **Multi-account parallel sync** - sync all accounts simultaneously
- **Non-blocking IMAP** - don't block the main thread
- **Better queue handling** - timeouts, retries, select
- **Future features** - HTTP API, WebSocket notifications, background daemon

## Current Architecture

```
┌─────────────────────────────────────────┐
│           Main Thread (Tauri)           │
├─────────────────────────────────────────┤
│  std::thread::spawn                     │
│  └── IMAP Queue Worker                  │
│      └── mpsc::channel                  │
│      └── Blocking IMAP calls            │
└─────────────────────────────────────────┘
```

## Target Architecture

```
┌─────────────────────────────────────────┐
│         Tokio Runtime (multi-thread)    │
├─────────────────────────────────────────┤
│  tokio::spawn                           │
│  ├── IMAP Queue Worker                  │
│  │   └── tokio::sync::mpsc              │
│  │   └── async-imap calls               │
│  ├── Account Sync Tasks (parallel)      │
│  │   └── One task per account           │
│  ├── Background Poller (future)         │
│  │   └── tokio::time::interval          │
│  └── HTTP API Server (future)           │
│      └── axum                           │
└─────────────────────────────────────────┘
```

## Dependencies

```toml
[dependencies]
tokio = { version = "1", features = ["full"] }
async-native-tls = "0.5"
async-imap = "0.9"
futures = "0.3"
```

## Migration Steps

### Phase 1: Add Tokio Runtime

1. Add tokio dependency with `rt-multi-thread` feature
2. Initialize runtime in Tauri setup
3. Keep existing sync code working

```rust
// src-tauri/src/lib.rs
use tokio::runtime::Runtime;
use std::sync::OnceLock;

static RUNTIME: OnceLock<Runtime> = OnceLock::new();

fn get_runtime() -> &'static Runtime {
    RUNTIME.get_or_init(|| {
        Runtime::new().expect("Failed to create tokio runtime")
    })
}

#[tauri::command]
fn sync_emails(account: String, mailbox: String) -> Result<SyncResult, String> {
    get_runtime().block_on(async {
        do_sync_emails(&account, &mailbox).await
    }).map_err(|e| e.to_string())
}
```

### Phase 2: Migrate IMAP Queue

Replace std mpsc with tokio mpsc:

```rust
// src-tauri/src/imap_queue.rs
use tokio::sync::mpsc;

static QUEUE_SENDER: OnceLock<mpsc::UnboundedSender<ImapOperation>> = OnceLock::new();

fn init_queue() -> mpsc::UnboundedSender<ImapOperation> {
    let (tx, mut rx) = mpsc::unbounded_channel::<ImapOperation>();

    get_runtime().spawn(async move {
        let mut pending: HashMap<String, ImapOperation> = HashMap::new();
        let mut interval = tokio::time::interval(Duration::from_millis(100));

        loop {
            tokio::select! {
                Some(op) = rx.recv() => {
                    let key = op.key();
                    pending.insert(key, op);
                }
                _ = interval.tick() => {
                    if !pending.is_empty() {
                        let ops: Vec<_> = pending.drain().collect();
                        for (_, op) in ops {
                            process_operation(op).await;
                        }
                    }
                }
            }
        }
    });

    tx
}
```

### Phase 3: Async IMAP Operations

Replace blocking `imap` crate with `async-imap`:

```rust
// src-tauri/src/imap_async.rs
use async_imap::Session;
use async_native_tls::TlsStream;
use tokio::net::TcpStream;

async fn connect_imap(account: &Account) -> Result<Session<TlsStream<TcpStream>>> {
    let creds = &account.credentials;
    let tcp = TcpStream::connect((creds.imap_host.as_str(), creds.imap_port)).await?;
    let tls = async_native_tls::TlsConnector::new();
    let tls_stream = tls.connect(&creds.imap_host, tcp).await?;
    let client = async_imap::Client::new(tls_stream);
    let session = client.login(&creds.email, &creds.password).await
        .map_err(|e| e.0)?;
    Ok(session)
}

async fn mark_read_imap(account: &str, mailbox: &str, uid: u32, unread: bool) -> Result<()> {
    let account = get_account(account)?;
    let mut session = connect_imap(&account).await?;
    session.select(mailbox).await?;

    let uid_set = format!("{}", uid);
    if unread {
        session.uid_store(&uid_set, "-FLAGS (\\Seen)").await?;
    } else {
        session.uid_store(&uid_set, "+FLAGS (\\Seen)").await?;
    }

    session.logout().await?;
    Ok(())
}
```

### Phase 4: Parallel Account Sync

Sync all accounts concurrently:

```rust
// src-tauri/src/sync.rs
use futures::future::join_all;

pub async fn sync_all_accounts() -> Result<Vec<SyncResult>> {
    let accounts = get_accounts()?;

    let tasks: Vec<_> = accounts.iter().map(|account| {
        let account_name = account.name.clone();
        async move {
            // Sync INBOX and other common mailboxes
            let mailboxes = vec!["INBOX", "Sent", "Drafts"];
            for mailbox in mailboxes {
                if let Err(e) = sync_mailbox(&account_name, mailbox).await {
                    eprintln!("Failed to sync {}/{}: {}", account_name, mailbox, e);
                }
            }
        }
    }).collect();

    join_all(tasks).await;
    Ok(vec![])
}

async fn sync_mailbox(account: &str, mailbox: &str) -> Result<SyncResult> {
    // ... async version of current sync_emails
}
```

### Phase 5: Tauri Async Commands

Use Tauri's async command support:

```rust
#[tauri::command]
async fn sync_emails(account: String, mailbox: String) -> Result<SyncResult, String> {
    do_sync_emails(&account, &mailbox).await
        .map_err(|e| e.to_string())
}

#[tauri::command]
async fn sync_all() -> Result<(), String> {
    sync_all_accounts().await
        .map_err(|e| e.to_string())?;
    Ok(())
}
```

## File Changes

```
src-tauri/
├── Cargo.toml              # Add tokio, async-imap, async-native-tls
├── src/
│   ├── lib.rs              # Add runtime initialization
│   ├── imap_queue.rs       # Migrate to tokio::sync::mpsc
│   ├── imap_async.rs       # New: async IMAP operations
│   ├── sync.rs             # New: parallel sync logic
│   └── mail.rs             # Keep sync versions for compatibility
```

## Rollout Strategy

1. **Phase 1-2**: Add tokio, keep existing code working
2. **Test**: Verify no regressions
3. **Phase 3-4**: Migrate IMAP to async
4. **Test**: Verify sync still works
5. **Phase 5**: Add parallel sync, new commands
6. **Test**: Verify multi-account sync

## Future Features (enabled by tokio)

### Background Polling + Notifications

Sync all accounts every 5 minutes and notify user of new emails:

```rust
use tauri::Manager;

async fn background_poller(app_handle: tauri::AppHandle) {
    let mut interval = tokio::time::interval(Duration::from_secs(300)); // 5 min

    loop {
        interval.tick().await;

        let accounts = match get_accounts() {
            Ok(a) => a,
            Err(_) => continue,
        };

        for account in accounts {
            let mailboxes = vec!["INBOX"];
            for mailbox in mailboxes {
                match sync_mailbox(&account.name, mailbox).await {
                    Ok(result) if result.new_emails > 0 => {
                        // Notify user of new emails
                        notify_new_emails(&app_handle, &account.name, result.new_emails);

                        // Emit event to frontend to refresh UI
                        let _ = app_handle.emit_all("emails:new", serde_json::json!({
                            "account": account.name,
                            "mailbox": mailbox,
                            "count": result.new_emails
                        }));
                    }
                    Err(e) => {
                        eprintln!("Sync failed for {}/{}: {}", account.name, mailbox, e);
                    }
                    _ => {}
                }
            }
        }
    }
}

fn notify_new_emails(app_handle: &tauri::AppHandle, account: &str, count: usize) {
    use tauri_plugin_notification::NotificationExt;

    let title = if count == 1 {
        "New email".to_string()
    } else {
        format!("{} new emails", count)
    };

    app_handle
        .notification()
        .builder()
        .title(&title)
        .body(&format!("in {}", account))
        .show()
        .unwrap_or_else(|e| eprintln!("Failed to show notification: {}", e));
}
```

**Frontend listener:**

```typescript
// src/App.tsx or Home.tsx
import { listen } from "@tauri-apps/api/event";

useEffect(() => {
  const unlisten = listen<{ account: string; mailbox: string; count: number }>(
    "emails:new",
    (event) => {
      // Refresh email list if viewing the same account/mailbox
      if (event.payload.account === selectedAccount &&
          event.payload.mailbox === selectedMailbox) {
        handleRefresh();
      }
      // Could also show a toast notification in-app
    }
  );

  return () => {
    unlisten.then((fn) => fn());
  };
}, [selectedAccount, selectedMailbox]);
```

**Initialize poller in Tauri setup:**

```rust
// src-tauri/src/lib.rs
pub fn run() {
    tauri::Builder::default()
        .plugin(tauri_plugin_notification::init())
        .setup(|app| {
            let handle = app.handle().clone();
            get_runtime().spawn(async move {
                background_poller(handle).await;
            });
            Ok(())
        })
        .invoke_handler(...)
        .run(tauri::generate_context!())
        .expect("error while running tauri application");
}
```

### Timeout Handling

```rust
async fn sync_with_timeout(account: &str, mailbox: &str) -> Result<SyncResult> {
    tokio::time::timeout(
        Duration::from_secs(30),
        sync_mailbox(account, mailbox)
    ).await
    .map_err(|_| "Sync timed out")?
}
```

### Retry with Backoff

```rust
async fn sync_with_retry(account: &str, mailbox: &str) -> Result<SyncResult> {
    let mut delay = Duration::from_secs(1);
    for attempt in 1..=3 {
        match sync_mailbox(account, mailbox).await {
            Ok(result) => return Ok(result),
            Err(e) if attempt < 3 => {
                eprintln!("Attempt {} failed: {}, retrying...", attempt, e);
                tokio::time::sleep(delay).await;
                delay *= 2;
            }
            Err(e) => return Err(e),
        }
    }
    unreachable!()
}
```

### HTTP API (future)

```rust
// Using axum
async fn start_api_server() {
    let app = axum::Router::new()
        .route("/emails", get(list_emails))
        .route("/sync", post(trigger_sync));

    axum::Server::bind(&"127.0.0.1:3000".parse().unwrap())
        .serve(app.into_make_service())
        .await
        .unwrap();
}
```

## Performance Considerations

- **Connection pooling**: Reuse IMAP connections instead of connect/disconnect per operation
- **Batch operations**: Group multiple UID operations into single IMAP command
- **Concurrent limits**: Use `tokio::sync::Semaphore` to limit concurrent connections
- **Graceful shutdown**: Handle app close by flushing pending operations

## Testing

```rust
#[tokio::test]
async fn test_parallel_sync() {
    let results = sync_all_accounts().await.unwrap();
    assert!(results.len() > 0);
}

#[tokio::test]
async fn test_queue_deduplication() {
    // Queue multiple toggles rapidly
    for _ in 0..10 {
        queue_mark_read("account", "INBOX", 123, true);
        queue_mark_read("account", "INBOX", 123, false);
    }
    // Only last state should be sent to IMAP
}
```
